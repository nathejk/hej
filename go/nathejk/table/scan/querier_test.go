package scan

import (
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func byTeamRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"qrId", "uts", "scannerId", "latitude", "longitude",
		"checkpointId", "name", "checkgroupId",
		"openFromUts", "openUntilUts", "openDuration",
		"scheme", "relativeCheckgroupId",
	})
}

func TestByTeamAttributesAndCarriesTheWindow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM scan s")).
		WithArgs("2026", "team-9").
		WillReturnRows(byTeamRows().
			AddRow("qr-1", 1750000500, "user-3", "56.1382", "9.5521",
				"cp-1", "Post 1", "cg-1", 1750000000, 1750003600, 0, "fixed", ""))

	got, err := querier{db: db}.ByTeam("2026", "team-9")
	if err != nil {
		t.Fatalf("ByTeam: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 scan, got %d", len(got))
	}
	s := got[0]
	if s.CheckpointName != "Post 1" || s.CheckgroupID != "cg-1" {
		t.Errorf("attribution: %+v", s)
	}
	if s.OpenFromUts != 1750000000 || s.OpenUntilUts != 1750003600 || s.Scheme != "fixed" {
		t.Errorf("window did not travel with the scan: %+v", s)
	}
	if s.Lat == nil || *s.Lat != 56.1382 {
		t.Errorf("position: %+v", s.Lat)
	}
}

// The property the LEFT JOIN exists for. An unattributable scan happened — the patrol was there and the
// post scanned their code — and the only thing missing is our ability to say where. Dropping it would
// turn an upstream rota gap into "the app lost our scan".
func TestByTeamKeepsUnattributedScans(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM scan s")).
		WithArgs("2026", "team-9").
		WillReturnRows(byTeamRows().
			AddRow("qr-2", 1750009999, "user-unknown", "", "", "", "", "", 0, 0, 0, "", ""))

	got, err := querier{db: db}.ByTeam("2026", "team-9")
	if err != nil {
		t.Fatalf("ByTeam: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("an unattributed scan must still be listed, got %d", len(got))
	}
	if got[0].CheckpointID != "" || got[0].CheckpointName != "" {
		t.Errorf("want no attribution, got %+v", got[0])
	}
	if got[0].Scheme != "" {
		t.Error("no checkpoint means no scheme, and therefore no verdict")
	}
}

// The join must be a LEFT JOIN in all three steps, or an unattributable scan disappears. Asserted on the
// query text because the behaviour above can be satisfied by a fixture while the real query still drops
// rows.
func TestByTeamUsesLeftJoins(t *testing.T) {
	if strings.Count(byTeamQuery, "LEFT JOIN") != 3 {
		t.Errorf("want three LEFT JOINs (shift, checkpoint, checkgroup)\ngot: %s", byTeamQuery)
	}
	if strings.Contains(byTeamQuery, "INNER JOIN") {
		t.Errorf("an INNER JOIN would silently drop unattributable scans\ngot: %s", byTeamQuery)
	}
}

// The attribution predicate, pinned. It is copied from hq's live implementation deliberately: the app and
// the organizers' own screens must agree about which post a scan happened at. Inclusive bounds, matching
// hq exactly.
func TestAttributionPredicateMatchesUpstream(t *testing.T) {
	for _, want := range []string{
		"cpn.userId = s.scannerId",
		"s.uts >= cpn.startUts",
		"s.uts <= cpn.endUts",
	} {
		if !strings.Contains(byTeamQuery, want) {
			t.Errorf("attribution predicate is missing %q\ngot: %s", want, byTeamQuery)
		}
	}
}

// 0,0 is the Atlantic off Ghana and is what an unset coordinate serialises to. A scan there must be
// listed but not plotted, which the client already distinguishes.
func TestZeroPositionIsNoPosition(t *testing.T) {
	lat, lng := parsePosition("0", "0")
	if lat != nil || lng != nil {
		t.Errorf("0,0 must be no position, got %v,%v", lat, lng)
	}
}

// An unparseable position is treated exactly like an absent one. It is a scan either way, and refusing
// the row over a bad string would lose the registration.
func TestUnparseablePositionIsNoPosition(t *testing.T) {
	for _, c := range [][2]string{{"", ""}, {"56.1", ""}, {"nope", "9.5"}, {"56.1", "nope"}} {
		lat, lng := parsePosition(c[0], c[1])
		if lat != nil || lng != nil {
			t.Errorf("parsePosition(%q, %q) = %v,%v; want nil,nil", c[0], c[1], lat, lng)
		}
	}
}

func TestParsePositionAcceptsAValidPair(t *testing.T) {
	lat, lng := parsePosition("56.1382", "9.5521")
	if lat == nil || lng == nil {
		t.Fatal("want a position")
	}
	if *lat != 56.1382 || *lng != 9.5521 {
		t.Errorf("got %v,%v", *lat, *lng)
	}
}

// The diagnostic task 260 logs. A year with a healthy rota reports a small number; a year with no shifts
// reports every scan, which is the shape of the disaster that would otherwise be invisible.
func TestUnattributedCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM scan s")).
		WithArgs("2026").
		WillReturnRows(sqlmock.NewRows([]string{"unattributed", "total"}).AddRow(40, 900))

	unattributed, total, err := querier{db: db}.UnattributedCount("2026")
	if err != nil {
		t.Fatalf("UnattributedCount: %v", err)
	}
	if unattributed != 40 || total != 900 {
		t.Errorf("got %d of %d", unattributed, total)
	}
}

// An empty scan table makes SUM() return NULL, not 0 — the classic aggregate trap. Scanned into a
// NullInt64 for that reason; without it, a fresh year would fail the diagnostic query rather than
// reporting zero.
func TestUnattributedCountOnAnEmptyTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM scan s")).
		WithArgs("2026").
		WillReturnRows(sqlmock.NewRows([]string{"unattributed", "total"}).AddRow(nil, 0))

	unattributed, total, err := querier{db: db}.UnattributedCount("2026")
	if err != nil {
		t.Fatalf("a fresh year must not fail the diagnostic: %v", err)
	}
	if unattributed != 0 || total != 0 {
		t.Errorf("got %d of %d", unattributed, total)
	}
}
