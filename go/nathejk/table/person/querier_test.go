package person

import (
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// testNormalizer mirrors internal/phone's rules closely enough to exercise the
// consistency property: strip non-digits, accept 8-digit Danish numbers, prefix +45.
//
// It is deliberately NOT a copy of the real implementation — the point of the
// injected port is that the projector and the lookup share whatever is passed in, so
// the test only needs the two sides to agree with *each other*.
type testNormalizer struct{}

func (testNormalizer) Normalize(input string) (string, error) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, input)

	switch {
	// 00<code><digits> — the international prefix real people still type. Handled
	// before the +45 case, since "0045..." also starts with digits that look local.
	case strings.HasPrefix(digits, "00") && len(digits) > 4:
		return "+" + strings.TrimPrefix(digits, "00"), nil
	case strings.HasPrefix(digits, "45") && len(digits) == 10:
		return "+" + digits, nil
	case len(digits) == 8:
		return "+45" + digits, nil
	default:
		return "", errors.New("invalid phone number")
	}
}

// This is the bug PRD 006 §2 warns about, expressed as a test: a number stored in
// one form must be found when a user types it in another. If the projector and the
// lookup ever fold numbers differently, the lookup returns nothing and the user is
// simply told their number is unrecognised — with nothing in the logs to explain it.
//
// The messy inputs are the ones real people type.
func TestLookupNormalizesMessyInput(t *testing.T) {
	stored := "+4530112233"

	for _, typed := range []string{
		"30112233",
		"3011 2233",
		"30 11 22 33",
		"+45 30 11 22 33",
		"+4530112233",
		"0045 30112233",
		"(30) 11-22-33",
	} {
		t.Run(typed, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			defer db.Close()

			// The assertion is in the argument: whatever the user typed, the query
			// must go out with the canonical form the projector would have written.
			mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
				WithArgs("2026", stored).
				WillReturnRows(personRows(stored))

			q := querier{db: db, normalizer: testNormalizer{}}
			got, err := q.Lookup("2026", typed)
			if err != nil {
				t.Fatalf("Lookup: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("want 1 match, got %d", len(got))
			}
			if got[0].Phone != stored {
				t.Fatalf("phone = %q, want %q", got[0].Phone, stored)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// An unparseable number must not reach the database at all. Otherwise it normalizes
// to "" and matches every row whose phone column holds the default empty string —
// i.e. a junk input would log someone in as an arbitrary person with no number on
// file.
func TestLookupWithUnparseableInputDoesNotQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	// No ExpectQuery: any query at all is a failure.

	q := querier{db: db, normalizer: testNormalizer{}}
	got, err := q.Lookup("2026", "not a phone number")
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if got != nil {
		t.Fatalf("want no matches, got %d", len(got))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Both owners of a shared number come back, in a stable order — the shape task 071's
// disambiguation flow depends on.
func TestLookupReturnsAllOwnersOfASharedNumber(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := personRows("+4530112233")
	addPersonRow(rows, "person-b", "Sibling B", "+4530112233")
	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs("2026", "+4530112233").
		WillReturnRows(rows)

	q := querier{db: db, normalizer: testNormalizer{}}
	got, err := q.Lookup("2026", "30112233")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 owners, got %d", len(got))
	}
}

func TestGetByIDReturnsFalseWhenAbsent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs("2026", "nobody").
		WillReturnRows(sqlmock.NewRows(personColumnNames()))

	q := querier{db: db, normalizer: testNormalizer{}}
	_, found, err := q.Get("2026", "nobody")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Fatal("want found=false for a missing person")
	}
}

// A guardian number that is NULL must scan as nil, not "", so "not applicable" stays
// distinguishable from "missing" (PRDs 003/005).
func TestNullGuardianPhoneScansAsNil(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs("2026", "+4530112233").
		WillReturnRows(personRows("+4530112233"))

	q := querier{db: db, normalizer: testNormalizer{}}
	got, err := q.Lookup("2026", "+4530112233")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got[0].PhoneParent != nil {
		t.Fatalf("want nil guardian phone, got %q", *got[0].PhoneParent)
	}
}

func personColumnNames() []string {
	return []string{
		"personId", "year", "appRole",
		"name", "phone", "phoneParent",
		"address", "postalCode", "city", "email", "birthday",
		"teamId", "teamName",
		"sectionSlug", "sectionName",
		"memberStatus", "armNumber",
		"verifiedAt", "acknowledgedPhone", "portraitRef", "portraitThumbRef", "portraitCapturedAt",
	}
}

func personRows(phone string) *sqlmock.Rows {
	rows := sqlmock.NewRows(personColumnNames())
	addPersonRow(rows, "person-a", "Sibling A", phone)
	return rows
}

// addPersonRow appends a row sized from personColumnNames, filling everything the test
// does not care about with a zero value.
//
// Built by name rather than as a positional literal because the positional version has
// now broken twice on an additive schema change (portraitCapturedAt, portraitThumbRef) —
// each time with a "expected 21 destination arguments, not 22" that says nothing about
// which column is missing. This way a new column costs nothing here, and a *renamed* one
// still fails loudly in scanPerson where it should.
func addPersonRow(rows *sqlmock.Rows, personID, name, phone string) {
	// The nullable columns must be nil rather than "": the querier scans them into
	// pointers, and the nil-vs-empty distinction is load-bearing for phoneParent.
	nullable := map[string]bool{
		"phoneParent": true, "birthday": true,
		"verifiedAt": true, "acknowledgedPhone": true, "portraitCapturedAt": true,
	}

	columns := personColumnNames()
	values := make([]driver.Value, 0, len(columns))
	for _, column := range columns {
		switch {
		case nullable[column]:
			values = append(values, nil)
		case column == "personId":
			values = append(values, personID)
		case column == "year":
			values = append(values, "2026")
		case column == "appRole":
			values = append(values, string(RoleSpejder))
		case column == "name":
			values = append(values, name)
		case column == "phone":
			values = append(values, phone)
		default:
			values = append(values, "")
		}
	}
	rows.AddRow(values...)
}
