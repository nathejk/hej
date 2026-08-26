package person

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// A guardian's number is not a credential. Decided 2026-08-26: nobody logs in with one
// — not the guardian, and not the member it belongs to.
//
// This is enforced rather than documented because the pressure to break it is specific
// and will come back. 36 live 2026 spejder have no phone of their own but do have a
// guardian number on file (PRD 006 §11 Q13), so `OR phoneParent = ?` looks like a
// one-line rescue of 36 locked-out children. It is really a way for a parent's handset
// to authenticate as their child with no audit trail — and where siblings share the
// number, as an arbitrary one of them.
//
// The test inspects the WHERE clause specifically. `phoneParent` legitimately appears in
// the SELECT list (the profile page renders it), so a naive substring check over the
// whole statement would either pass vacuously or fail for the wrong reason.
func TestLoginNeverMatchesOnTheGuardianNumber(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(querier) error
	}{
		{"Lookup", func(q querier) error {
			_, err := q.Lookup("2026", "30112233")
			return err
		}},
		{"Get", func(q querier) error {
			_, _, err := q.Get("2026", "member-1")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var executed []string

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(
				sqlmock.QueryMatcherFunc(func(_, actualSQL string) error {
					executed = append(executed, actualSQL)
					return nil
				})))
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			defer db.Close()

			mock.ExpectQuery("").WillReturnRows(personRows("+4530112233"))

			if err := tc.call(querier{db: db, normalizer: testNormalizer{}}); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if len(executed) == 0 {
				t.Fatal("no statement executed")
			}

			for _, sql := range executed {
				where := whereClause(t, sql)
				if strings.Contains(strings.ToLower(where), "phoneparent") {
					t.Errorf("the guardian number must never be a lookup key.\n"+
						"A parent's handset would authenticate as their child.\n"+
						"WHERE clause: %s", where)
				}
			}
		})
	}
}

// whereClause returns everything after the first WHERE, which is where a match
// condition would have to live.
func whereClause(t *testing.T, sql string) string {
	t.Helper()
	loc := regexp.MustCompile(`(?is)\bWHERE\b`).FindStringIndex(sql)
	if loc == nil {
		t.Fatalf("statement has no WHERE clause, so it is unscoped: %s", sql)
	}
	return sql[loc[1]:]
}

// The same boundary from the other side: a lookup for a number that exists only as
// somebody's guardian number must find nobody.
//
// personRows is built with the guardian column set, and the query is expected to carry
// the *member's* number as its argument — so if the implementation ever widened to
// match phoneParent, the argument list would have to change and this expectation would
// fail.
func TestLookupQueriesOnlyTheOwnPhoneColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	// Exactly two arguments: the year and one number. A guardian-number fallback would
	// need a third, or a repeated one.
	mock.ExpectQuery(regexp.QuoteMeta("FROM person")).
		WithArgs("2026", "+4530112233").
		WillReturnRows(personRows("+4530112233"))

	q := querier{db: db, normalizer: testNormalizer{}}
	if _, err := q.Lookup("2026", "30112233"); err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
