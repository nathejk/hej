package person

import (
	"strings"
	"testing"
)

// The schema is embedded, so a rename or a missing file is a build-time problem
// rather than a runtime surprise. This pins the columns other PRDs depend on by
// name, since a silent rename would make a projector write to a column nothing
// reads.
func TestSchemaCarriesTheExpectedColumns(t *testing.T) {
	if strings.TrimSpace(tableSchema) == "" {
		t.Fatal("table.sql did not embed")
	}
	for _, col := range []string{
		"personId", "year", "appRole",
		"phone", "phoneParent",
		"teamId", "teamName",
		"memberStatus", "armNumber",
		"verifiedAt", "acknowledgedPhone", "portraitRef", "portraitCapturedAt",
		"deleted",
	} {
		if !strings.Contains(tableSchema, col) {
			t.Errorf("schema is missing column %q", col)
		}
	}
}

// The phone index must exist and must NOT be unique: two people can share a number,
// and a UNIQUE constraint would make the projector fail on real data instead of
// letting the collision policy decide (task 071).
func TestPhoneIndexIsNotUnique(t *testing.T) {
	if !strings.Contains(tableSchema, "KEY year_phone (year, phone)") {
		t.Error("expected a non-unique year_phone index for the login lookup")
	}
	if strings.Contains(tableSchema, "UNIQUE KEY year_phone") ||
		strings.Contains(tableSchema, "UNIQUE (year, phone)") {
		t.Error("year_phone must not be UNIQUE — see task 071 on phone collisions")
	}
}

// phoneParent must be nullable so "this population has no guardian number" is
// distinguishable from "should have one and it is missing". PRD 003 and PRD 005
// render those two differently.
func TestGuardianPhoneIsNullable(t *testing.T) {
	if !strings.Contains(tableSchema, "phoneParent VARCHAR(99) NULL DEFAULT NULL") {
		t.Error("phoneParent must be NULL-able, not defaulted to an empty string")
	}
}

// An empty phone must not reach the database. Without the guard it would match every
// row whose phone column holds the default empty string — i.e. a failed
// normalization would log someone in as an arbitrary person with no number on file.
//
// The querier is given a nil Reader deliberately: if the guard is ever removed, this
// test panics instead of quietly passing.
func TestLookupWithEmptyPhoneDoesNotQuery(t *testing.T) {
	q := querier{db: nil}

	got, err := q.Lookup("2026", "")
	if err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if got != nil {
		t.Fatalf("want no matches for an empty phone, got %d", len(got))
	}
}
