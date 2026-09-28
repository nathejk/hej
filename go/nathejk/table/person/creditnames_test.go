package person

import (
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// CreditNames: the photo credit resolver (PRD 025 §6 R3, task 451).
//
// # Why this file is the most guarded read in the package
//
// It is **the only path from the person projection to a public page.** PRD 022 §6 originally forbade it
// outright: the credit line was admitted as the one field naming a human being precisely *because* nothing
// derived it. PRD 025 reversed that for one reason — erasure, since a name copied into `photo.credit` is also on
// the append-only event log and could never be deleted — and the price of the reversal is that these bounds are
// now the whole of the guarantee:
//
//  1. crew roles only, so no participant's name can ever be published;
//  2. the name column only;
//  3. one year, the photograph's own;
//  4. absent means absent, which is how deleting a person erases their credit everywhere.
//
// Each has a test below, and each test says what would happen without it.

// The mock is `crewMock` from crewroster_test.go: the same capture of the actual statement, because these two
// reads have the same one risk and asserting it twice from two harnesses would be two harnesses to keep right.

// creditArgs is what CreditNames binds: the year, the ids, then every role in `CrewRoles`.
//
// Built from `CrewRoles` rather than written out, so that changing the union is visible in one place and a test
// cannot pin a narrower set than the code uses.
func creditArgs(year string, ids ...string) []driver.Value {
	args := []driver.Value{year}
	for _, id := range ids {
		args = append(args, id)
	}
	for _, role := range CrewRoles {
		args = append(args, role)
	}
	return args
}

func TestCreditNamesResolvesACrewMember(t *testing.T) {
	q, mock, _ := crewMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs(creditArgs("2026", "user-1", "user-2")...).
		WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}).
			AddRow("user-1", "Anne Sørensen").
			AddRow("user-2", "Bo Hansen"))

	got, err := q.CreditNames("2026", []string{"user-1", "user-2"})
	if err != nil {
		t.Fatalf("CreditNames: %v", err)
	}
	if got["user-1"] != "Anne Sørensen" || got["user-2"] != "Bo Hansen" {
		t.Errorf("unexpected names: %v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// **Bound 1, and the one that matters.** The statement filters on `appRole`, against every crew role and nothing
// else.
//
// Without it a credit id could name a spejder — and a spejder's name on a public page is the single thing this
// service's privacy posture is arranged to prevent (PRD 011 §0b.1, PRD 022 §6). The id would not even have to be
// malicious: ids are reissued between years (the maintainer, 2026-09-28), so last year's crew id can be this
// year's participant.
func TestCreditNamesResolvesOnlyCrew(t *testing.T) {
	q, mock, seen := crewMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs(creditArgs("2026", "user-1")...).
		WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}))

	if _, err := q.CreditNames("2026", []string{"user-1"}); err != nil {
		t.Fatalf("CreditNames: %v", err)
	}

	lower := strings.ToLower(*seen)
	if lower == "" {
		t.Fatal("no statement was captured: this test would otherwise pass while asserting nothing")
	}
	want := "approle in (" + strings.TrimSuffix(strings.Repeat("?, ", len(CrewRoles)), ", ") + ")"
	if !strings.Contains(lower, want) {
		t.Errorf("the resolver must filter on appRole against every crew role (%q):\n%s", want, *seen)
	}
	// And the roles are bound, not spliced — the expectation above proves which, this proves they are
	// parameters. A spliced role list is a statement somebody can widen with a string.
	if strings.Contains(lower, "'"+RoleSpejder+"'") || strings.Contains(lower, RoleSpejder) {
		t.Errorf("no role may appear in the statement text:\n%s", *seen)
	}
}

// **Bound 2.** The statement names `personId` and `name`, and no other column of the projection.
//
// Checked against the whole column list rather than a hand-written blocklist, so a column added to table.sql
// tomorrow is covered without anybody remembering this file. The row holds a phone number, a guardian's phone
// number, an email, an address and a birthday; this read is one `SELECT` away from any of them, and the line it
// would be widened on is the one that already looks harmless.
func TestCreditNamesSelectsOnlyAnIdAndAName(t *testing.T) {
	q, mock, seen := crewMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs(creditArgs("2026", "user-1")...).
		WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}).AddRow("user-1", "Anne"))

	if _, err := q.CreditNames("2026", []string{"user-1"}); err != nil {
		t.Fatalf("CreditNames: %v", err)
	}

	lower := strings.ToLower(*seen)
	selectClause := lower
	if i := strings.Index(lower, "from person"); i >= 0 {
		selectClause = lower[:i]
	}
	for _, column := range strings.Split(strings.ReplaceAll(personColumns, "\n", ","), ",") {
		column = strings.TrimSpace(column)
		switch strings.ToLower(column) {
		case "", "personid", "name":
			continue
		}
		if strings.Contains(selectClause, strings.ToLower(column)) {
			t.Errorf("the resolver selects %q. It may read a crew member's name and nothing else:\n%s",
				column, *seen)
		}
	}
}

// **Bound 3.** One year, and the year is bound.
//
// Nothing in this service crosses a year, and a credit is resolved against the **photograph's** year rather than
// the current one — so somebody who was crew in 2026 and is not in 2027 keeps their 2026 credits. An empty year
// would match the column default rather than "every year", which is why it short-circuits instead.
func TestCreditNamesIsScopedToOneYear(t *testing.T) {
	q, mock, seen := crewMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs(creditArgs("2026", "user-1")...).
		WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}).AddRow("user-1", "Anne"))

	if _, err := q.CreditNames("2026", []string{"user-1"}); err != nil {
		t.Fatalf("CreditNames: %v", err)
	}
	if !strings.Contains(strings.ToLower(*seen), "year = ?") {
		t.Errorf("the resolver must filter on the year:\n%s", *seen)
	}

	// No year, no query at all.
	q2, mock2, seen2 := crewMock(t)
	got, err := q2.CreditNames("", []string{"user-1"})
	if err != nil || len(got) != 0 {
		t.Errorf("an empty year must resolve nothing, got %v (%v)", got, err)
	}
	if *seen2 != "" {
		t.Errorf("an empty year must not reach the database:\n%s", *seen2)
	}
	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// **Bound 4.** Absent is absent: a deleted crew member, or one who is not crew, is simply missing from the map.
//
// This is the erasure path (PRD 025 §6 R4). Deleting the person row stops every photograph they took from being
// credited, with nothing to find and nothing to rewrite — which is the entire reason the credit is a reference
// rather than a copied name. The caller renders no credit line for a missing id (R5).
func TestCreditNamesLeavesOutWhatItCannotResolve(t *testing.T) {
	q, mock, seen := crewMock(t)

	// Two ids asked for, one row back: the other is deleted, not crew, or gone.
	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs(creditArgs("2026", "user-1", "user-gone")...).
		WillReturnRows(sqlmock.NewRows([]string{"personId", "name"}).AddRow("user-1", "Anne"))

	got, err := q.CreditNames("2026", []string{"user-1", "user-gone"})
	if err != nil {
		t.Fatalf("CreditNames: %v", err)
	}
	if len(got) != 1 || got["user-1"] != "Anne" {
		t.Errorf("want only the resolvable id, got %v", got)
	}
	if _, present := got["user-gone"]; present {
		t.Error("an unresolvable id must be absent from the map, not present as an empty string: the caller " +
			"renders no credit line for a missing id, and an empty one would be a credit that says nothing")
	}

	// `deleted = 0` is what makes deletion erase the credit, so it is asserted rather than assumed.
	if !strings.Contains(strings.ToLower(*seen), "deleted = 0") {
		t.Errorf("the resolver must exclude deleted rows, or deleting a crew member would not remove their "+
			"name from the photographs:\n%s", *seen)
	}
	// A nameless stub row (see handleSectionAssigned) must not resolve either: a credit line of "" is a
	// photograph credited to nobody, rendered as if somebody had been named.
	if !strings.Contains(*seen, `name <> ""`) {
		t.Errorf("the resolver must skip rows with no name yet:\n%s", *seen)
	}
}

// No ids, no query. The common case on a page of photographs where nobody picked a crew member, and it must not
// cost a round trip.
func TestCreditNamesWithNothingToResolveDoesNotQuery(t *testing.T) {
	for name, ids := range map[string][]string{
		"no ids":    nil,
		"empty ids": {"", ""},
	} {
		q, mock, seen := crewMock(t)
		got, err := q.CreditNames("2026", ids)
		if err != nil || len(got) != 0 {
			t.Errorf("%s: want an empty result, got %v (%v)", name, got, err)
		}
		if *seen != "" {
			t.Errorf("%s: must not reach the database:\n%s", name, *seen)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// The union itself, since both the picker and the resolver rest on it.
func TestIsCrewCoversStaffAndNoParticipant(t *testing.T) {
	for _, role := range CrewRoles {
		if !IsCrew(role) {
			t.Errorf("%q is in CrewRoles but IsCrew says no", role)
		}
	}
	// The three that must never be pickable or resolvable. Gøglere are the one worth naming: staff-adjacent,
	// and participant-side here — `MayLookUpPatrol` refuses them because "the lookup exists for a safety task,
	// not a game one", and the credit picker inherits that.
	for _, role := range []string{RoleSpejder, RoleBandit, RoleGoegler} {
		if IsCrew(role) {
			t.Errorf("%q must not count as crew: a participant's name may never be published", role)
		}
	}
	// An unknown role is not crew. A value from a newer binary, or a stub row with no role yet, must not be
	// treated as staff — the safe answer to "is this person staff" is no.
	for _, role := range []string{"", "photographer", "crewmember", "CREW"} {
		if IsCrew(role) {
			t.Errorf("%q is not a role this package knows and must not count as crew", role)
		}
	}
}
