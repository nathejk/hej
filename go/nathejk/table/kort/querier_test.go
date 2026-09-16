package kort

import (
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func patrolSheetRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "kortsaetId", "name", "format", "sortOrder",
		"handoutCheckgroupId", "checkpointIds", "extents",
	})
}

// The filter is bound to the team type `patrulje`, never to a set name. This is the guard against the
// mistake the field's documentation warns about at length: a set renamed mid-season ("Patruljer" →
// "Patruljekort") must not change what a patrol is handed. Pinned on the query text and the bound arg
// because a name comparison would look entirely reasonable in review and would break quietly, at night,
// in the middle of a race.
func TestPatrolSheetsFiltersOnTeamTypeNotName(t *testing.T) {
	if !strings.Contains(patrolSheetsQuery, "s.teamType = ?") {
		t.Errorf("patrol sheets must be filtered on the set's team type\ngot: %s", patrolSheetsQuery)
	}

	// The name column is *selected* (k.name — the sheet's own name), but the set's name must never appear
	// as a filter. A rename would then be invisible to this query, which is the whole point.
	where := patrolSheetsQuery[strings.Index(patrolSheetsQuery, "WHERE"):]
	for _, banned := range []string{"s.name", ".name =", "name ="} {
		if strings.Contains(where, banned) {
			t.Errorf("patrol sheets must not be filtered on a name\nWHERE: %s", where)
		}
	}

	// The value must be `patrulje` and never `spejder` — spejder is the domain's word for a person, HQ
	// refuses it on write, and a filter against it would reveal an empty map while looking correct.
	if string(PatrolTeamType) != "patrulje" {
		t.Errorf("patrol sets are filtered on %q, want patrulje", PatrolTeamType)
	}
}

// The bound argument is the team type, confirmed against a live call so the const and the Query() call
// cannot drift apart.
func TestPatrolSheetsBindsTheTeamType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("s.teamType = ?")).
		WithArgs("2026", string(PatrolTeamType)).
		WillReturnRows(patrolSheetRows())

	if _, err := (querier{db: db}).PatrolSheets("2026"); err != nil {
		t.Fatalf("PatrolSheets: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("query did not bind teamType %q: %v", PatrolTeamType, err)
	}
}

// teamType is a filter yielding candidate sheets, not a key: a year may legitimately split its patrol maps
// across two sets, and both must appear. Two rows from two different kortsaetIds come back as two sheets —
// nothing collapses the result to a single set.
func TestPatrolSheetsReturnsSheetsFromAllMatchingSets(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM kort k")).
		WithArgs("2026", string(PatrolTeamType)).
		WillReturnRows(patrolSheetRows().
			AddRow("kort-1", "saet-nord", "Etape 1", "a4", 0, "", "[]", "[]").
			AddRow("kort-2", "saet-syd", "Etape 2", "a4", 0, "", "[]", "[]"))

	got, err := (querier{db: db}).PatrolSheets("2026")
	if err != nil {
		t.Fatalf("PatrolSheets: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want sheets from both sets, got %d", len(got))
	}
	if got[0].KortsaetID == got[1].KortsaetID {
		t.Errorf("both sheets came back under one set: %q", got[0].KortsaetID)
	}
}
