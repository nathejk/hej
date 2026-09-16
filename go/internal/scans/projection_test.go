package scans

import (
	"errors"
	"testing"
)

type stubProjection struct {
	rows []ProjectedScan
	err  error

	gotYear, gotTeam string
	calls            int
}

func (s *stubProjection) ByTeam(year, teamID string) ([]ProjectedScan, error) {
	s.calls++
	s.gotYear, s.gotTeam = year, teamID
	return s.rows, s.err
}

func f(v float64) *float64 { return &v }

func TestProjectionSourceMapsRows(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-1", Uts: 1750000500, CheckpointID: "cp-1", CheckpointName: "Post 1",
			Lat: f(56.1382), Lng: f(9.5521)},
	}}

	got := NewProjectionSource(p, "2026", nil, nil).ByPatrol("team-9")

	if len(got) != 1 {
		t.Fatalf("want 1 registration, got %d", len(got))
	}
	if got[0].Label != "Post 1" {
		t.Errorf("label: %q", got[0].Label)
	}
	if got[0].Kind != KindCheckpoint {
		t.Errorf("kind: %q", got[0].Kind)
	}
	if got[0].Lat == nil || *got[0].Lat != 56.1382 {
		t.Errorf("position: %v", got[0].Lat)
	}
	if p.gotYear != "2026" || p.gotTeam != "team-9" {
		t.Errorf("queried %q/%q", p.gotYear, p.gotTeam)
	}
}

// An unattributed scan is labelled "Registrering" — not blank, and not a guessed post name. Blank would
// read as a rendering bug; a guess would be a lie about where the patrol was.
func TestUnattributedScanIsLabelledHonestly(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{{QrID: "qr-2", Uts: 1750009999}}}

	got := NewProjectionSource(p, "2026", nil, nil).ByPatrol("team-9")

	if len(got) != 1 {
		t.Fatalf("an unattributed scan must still be listed, got %d", len(got))
	}
	if got[0].Label != "Registrering" {
		t.Errorf("want an honest label, got %q", got[0].Label)
	}
}

// The id must be stable across replays, because the client uses it as a list key. The event carries no
// scan id, so it is derived from the projection's key.
func TestRegistrationIDIsStable(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{{QrID: "qr-1", Uts: 1750000500}}}
	src := NewProjectionSource(p, "2026", nil, nil)

	first := src.ByPatrol("team-9")
	second := src.ByPatrol("team-9")

	if first[0].ID != second[0].ID {
		t.Errorf("id is not stable: %q vs %q", first[0].ID, second[0].ID)
	}
	if first[0].ID == "" {
		t.Error("id must not be empty")
	}
}

// Personnel roles have no patrol. Short-circuited rather than queried, because asking the database for the
// scans of team "" is a query per request that can only ever return nothing.
func TestNoPatrolDoesNotQuery(t *testing.T) {
	p := &stubProjection{}

	got := NewProjectionSource(p, "2026", nil, nil).ByPatrol("")

	if got != nil {
		t.Errorf("want no registrations, got %v", got)
	}
	if p.calls != 0 {
		t.Errorf("want no query for an empty patrol, got %d", p.calls)
	}
}

// `Source.ByPatrol` cannot return an error — the map must stay usable when registrations fail — so a
// failure must be *reported*, or a broken projection looks exactly like a patrol that has not scanned
// anything yet.
func TestFailureIsReportedNotSwallowed(t *testing.T) {
	p := &stubProjection{err: errors.New("database is down")}

	var reported error
	got := NewProjectionSource(p, "2026", nil, func(err error) { reported = err }).ByPatrol("team-9")

	if got != nil {
		t.Errorf("want no registrations on failure, got %v", got)
	}
	if reported == nil {
		t.Fatal("a failure must be reported")
	}
}

func TestNilReportIsAllowed(t *testing.T) {
	p := &stubProjection{err: errors.New("database is down")}

	// Must not panic: tests and the no-database mode construct the source without a sink.
	if got := NewProjectionSource(p, "2026", nil, nil).ByPatrol("team-9"); got != nil {
		t.Errorf("want no registrations, got %v", got)
	}
}
