package person

import (
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// The crew roster behind the photo credit picker (PRD 025 §6 R2, task 449).
//
// These tests are about the **statement**, not the rows, because that is where this read's one risk lives.
// The person row holds a telephone number, a guardian's number, an email, an address and a birthday; the
// guarantee that none of them reach a picker is that the SELECT never asks for them. A test over the scanned
// values would still pass on a query that fetched all of it and discarded it — and the next person to add a
// field would find it already in hand.

// crewMock returns a querier over a mock, plus the statement it was last asked to run.
//
// The statement is captured through sqlmock's query matcher rather than re-derived, because the matcher is
// the only place the *actual* SQL is available: an expectation reports whether a pattern matched, not what it
// matched against. Matching itself is delegated to the default regexp matcher, so `ExpectQuery` behaves as
// it does everywhere else in this package.
func crewMock(t *testing.T) (querier, sqlmock.Sqlmock, *string) {
	t.Helper()

	var seen string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(
		sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
			seen = actualSQL
			return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
		})))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return querier{db: db, normalizer: testNormalizer{}}, mock, &seen
}

// The load-bearing test: two columns, and the statement can name nothing else about a person.
//
// Asserted against the column list of the whole projection rather than a hand-written blocklist, so a column
// added to table.sql tomorrow is covered without anybody remembering this file. `personId` and `name` are the
// two exceptions, and they are the read's entire contract.
func TestCrewRosterSelectsOnlyAnIdAndAName(t *testing.T) {
	q, mock, seen := crewMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs("2026", RoleCrew, "pr").
		WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}).
			AddRow("user-1", "Anne Sørensen"))

	got, err := q.CrewRoster("2026", "pr")
	if err != nil {
		t.Fatalf("CrewRoster: %v", err)
	}
	if len(got) != 1 || got[0].PersonID != "user-1" || got[0].Name != "Anne Sørensen" {
		t.Fatalf("unexpected roster: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	lower := strings.ToLower(*seen)
	if lower == "" {
		t.Fatal("no statement was captured: this test would otherwise pass while asserting nothing")
	}
	selectClause := lower
	if i := strings.Index(lower, "from person"); i >= 0 {
		selectClause = lower[:i]
	}
	for _, column := range strings.Split(strings.ReplaceAll(personColumns, "\n", ","), ",") {
		column = strings.ToLower(strings.TrimSpace(column))
		switch column {
		case "", "personid", "name":
			continue
		}
		if strings.Contains(selectClause, column) {
			t.Errorf("the crew roster selects %q. This read is rendered into a picker, so every column it "+
				"fetches is one something is about to draw — and the person row holds a phone number, a "+
				"guardian's phone number, an email, an address and a birthday (PRD 025 §6 R2, and the hard "+
				"rule in .rules). An id and a name is the whole contract; widening it is a decision, not a "+
				"convenience", column)
		}
	}
}

// Only RoleCrew, which is the same rule the credit resolver applies (PRD 025 §6 R3).
//
// Two places deciding "who is crew" is how they come to disagree, and the visible symptom would be a picker
// offering a name the resolver then refuses to publish — a credit that silently does not appear.
func TestCrewRosterAsksForCrewAndNothingElse(t *testing.T) {
	q, mock, seen := crewMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs("2026", RoleCrew, "pr").
		WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}))

	if _, err := q.CrewRoster("2026", "pr"); err != nil {
		t.Fatalf("CrewRoster: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	// The role is a bound parameter, so the expectation above already proved *which* role. This is the other
	// half: that the role is filtered at all, rather than the section standing in for it. A spejder with a
	// `sectionSlug` — impossible today, and one upstream change away — would otherwise be in the picker.
	if !strings.Contains(*seen, "appRole = ?") {
		t.Errorf("the roster must filter on appRole:\n%s", *seen)
	}
}

// An empty section is the whole year's crew: `pr` is the default filter, not the boundary (PRD 025 §5).
func TestCrewRosterWithNoSectionReturnsTheWholeYearsCrew(t *testing.T) {
	q, mock, seen := crewMock(t)

	// Two arguments, not three: the section predicate must be absent rather than matching "". An empty
	// sectionSlug is the column default, so `sectionSlug = ""` would return the crew who have **no** section
	// — the opposite of "all of them".
	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs("2026", RoleCrew).
		WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}).AddRow("user-1", "Anne"))

	got, err := q.CrewRoster("2026", "")
	if err != nil {
		t.Fatalf("CrewRoster: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want the whole year's crew, got %d rows", len(got))
	}
	if strings.Contains(*seen, "sectionSlug") {
		t.Errorf("an unfiltered roster must not mention sectionSlug at all:\n%s", *seen)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The slug is folded before it reaches SQL, with the same normalizer the section comparison elsewhere uses.
//
// Two different foldings of one string is how a section silently has no crew — the failure task 300 exported
// `NormalizeSectionSlug` to prevent.
func TestCrewRosterFoldsTheSectionSlug(t *testing.T) {
	for _, asked := range []string{"pr", "PR", " Pr ", "pR\t"} {
		t.Run(asked, func(t *testing.T) {
			q, mock, _ := crewMock(t)

			mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
				WithArgs("2026", RoleCrew, "pr").
				WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}))

			if _, err := q.CrewRoster("2026", asked); err != nil {
				t.Fatalf("CrewRoster: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Live rows only, and named ones only.
//
// The deleted check is the other half of PRD 025 §6 R4: erasure works by deletion, so a removed crew member
// must stop being offered. The name check is about the stub row `handleSectionAssigned` writes when an
// assignment arrives before the member's details — a blank entry in a picker credits a photograph to nobody.
func TestCrewRosterSkipsDeletedAndNamelessRows(t *testing.T) {
	q, mock, seen := crewMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs("2026", RoleCrew, "pr").
		WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}))

	if _, err := q.CrewRoster("2026", "pr"); err != nil {
		t.Fatalf("CrewRoster: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(*seen, "deleted = 0") {
		t.Errorf("the roster must exclude soft-deleted rows: PRD 025 §6 R4 makes deletion the erasure "+
			"mechanism, so a removed crew member who is still offered means the deletion did nothing:\n%s",
			*seen)
	}
	if !strings.Contains(*seen, `name <> ""`) {
		t.Errorf("the roster must exclude nameless rows: a section assignment can land before the member's "+
			"details, and a blank option in a picker credits a photograph to nobody:\n%s", *seen)
	}
}

// An empty year returns nothing rather than every year's crew.
//
// The same guard `TrackMembers` puts on an empty team id: an empty year matches the column default, and
// nothing in this service crosses a year — least of all a credit, which resolves within the photograph's own
// year by PRD 025 §11 Q1.
func TestCrewRosterRefusesAnEmptyYear(t *testing.T) {
	q, mock, seen := crewMock(t)

	got, err := q.CrewRoster("", "pr")
	if err != nil || got != nil {
		t.Fatalf(`CrewRoster("") = %v, %v — want no rows and no query`, got, err)
	}
	// No statement reached the database at all, which is the assertion.
	if *seen != "" {
		t.Errorf("an empty year must not query:\n%s", *seen)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
