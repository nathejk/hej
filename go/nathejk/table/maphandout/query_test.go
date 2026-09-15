package maphandout

import (
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestByPatrolReturnsHistoryOldestFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM maphandout")).
		WithArgs("2026", "team-9").
		WillReturnRows(sqlmock.NewRows([]string{"qrId", "mapId", "firstUts", "lastUts", "current"}).
			AddRow("qr-1", "kort-1", 1750000000, 1750000000, true).
			AddRow("qr-2", "", 1750003600, 1750003600, false))

	got, err := querier{db: db}.ByPatrol("2026", "team-9")
	if err != nil {
		t.Fatalf("ByPatrol: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 handouts, got %d", len(got))
	}

	if got[0].QrID != "qr-1" || got[0].MapID != "kort-1" || !got[0].Current {
		t.Errorf("first handout: %+v", got[0])
	}

	// "" is *unknown sheet*, not *no sheet* — the patrol is holding something, and the caller renders
	// "Ukendt kort". Dropping the row would hide a sheet the patrol has in its hand.
	if got[1].MapID != "" {
		t.Errorf("want an unknown sheet preserved, got %q", got[1].MapID)
	}
	if got[1].Current {
		t.Error("a reassigned sheet must not read as current")
	}
}

// Empty is a normal answer, not an error: a patrol before its first handout, and every user without
// a patrol at all. A nil slice would serialise as `null` and make the client branch on it, so an
// empty slice is returned instead.
func TestByPatrolEmptyIsNotAnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM maphandout")).
		WithArgs("2026", "team-none").
		WillReturnRows(sqlmock.NewRows([]string{"qrId", "mapId", "firstUts", "lastUts", "current"}))

	got, err := querier{db: db}.ByPatrol("2026", "team-none")
	if err != nil {
		t.Fatalf("ByPatrol: %v", err)
	}
	if got == nil {
		t.Fatal("want an empty slice, not nil")
	}
	if len(got) != 0 {
		t.Fatalf("want no handouts, got %v", got)
	}
}

// The rule this projection exists to enforce, expressed as a test.
//
// HQ's equivalent read returns the successor team — id, number and name — so an organizer can chase a
// reassigned sheet. A participant may not see that: it names another team to a patrol with no
// business knowing. The defence is that our query never *selects* those columns, so there is nothing
// for a handler to forget to strip.
//
// Asserted against the query text rather than the result, because the result cannot show the absence
// of a column that nobody added yet. This is the test that should fail in six months when someone
// copies a join across from hq.
func TestByPatrolNeverSelectsTheSuccessorTeam(t *testing.T) {
	for _, forbidden := range []string{
		"teamNumber", // the successor's number
		"p.name",     // the successor's name, via a patrulje join
		"currentTeam",
		"JOIN patrulje",
	} {
		if strings.Contains(byPatrolQuery, forbidden) {
			t.Errorf("ByPatrol must not select %q: naming the team that now holds a reassigned "+
				"sheet is exactly what a participant may not see (PRD 016 §4)", forbidden)
		}
	}

	// And the positive half: it must still derive "do we hold it" from the newest binding, or the
	// "afleveret" state would be wrong rather than merely absent.
	for _, required := range []string{"ROW_NUMBER()", "cur.teamId = mh.teamId"} {
		if !strings.Contains(byPatrolQuery, required) {
			t.Errorf("ByPatrol must still derive current holding: missing %q", required)
		}
	}
}
