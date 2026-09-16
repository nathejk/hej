package scans

import (
	"time"
)

// Projection is the read API this package adapts, satisfied by nathejk/table/scan's querier.
//
// Declared here rather than importing the projection's own interface so that `internal/scans` keeps
// owning the shape its handlers need, and so the projection package stays free of any dependency on
// this one. The concrete projection is wired in main.go.
type Projection interface {
	ByTeam(year, teamID string) ([]ProjectedScan, error)
}

// ProjectedScan is one scan as the projection reports it.
//
// A near-copy of the projection's own row type. The duplication is deliberate: it is the seam that lets
// `internal/scans` depend on nothing under nathejk/table, and it is where "a row in a read model"
// becomes "a registration a patrol sees".
type ProjectedScan struct {
	QrID           string
	Uts            int64
	CheckpointID   string
	CheckpointName string
	Lat            *float64
	Lng            *float64

	// The attributed checkpoint's group, its window and the group's scheme — carried through so the
	// verdict is a pure function over the patrol's own scans (task 265). Zero and "" when the scan could
	// not be attributed to a post, which is exactly when there is no verdict to compute.
	CheckgroupID         string
	Scheme               string
	RelativeCheckgroupID string
	OpenFromUts          int64
	OpenUntilUts         int64
	OpenDurationMinutes  int
}

// NewProjectionSource returns a Source backed by the scan projection.
//
// # Why this exists rather than the handler reading the projection directly
//
// `Source` is the seam the mock was built behind, precisely so the real source could replace it without
// touching handlers (the package doc has said so since it was written). This is that replacement.
//
// # Errors become an empty list, and are reported
//
// `Source.ByPatrol` returns no error, by design: the map must stay usable when registrations cannot be
// fetched, and every caller would otherwise have to decide what to do about a failure it cannot fix.
// But an unreported database error would make a broken projection look exactly like a patrol that has
// not scanned anything yet — so failures go to the `report` sink instead of being swallowed. Nil report
// is allowed; tests use it.
func NewProjectionSource(p Projection, year string, report func(error)) Source {
	return projectionSource{p: p, year: year, report: report}
}

type projectionSource struct {
	p      Projection
	year   string
	report func(error)
}

// ByPatrol returns the patrol's registrations, newest first.
//
// An empty patrol id short-circuits: personnel roles have none, and asking the database for the scans of
// team "" would be a query per request that can only ever return nothing.
//
// # Every scan is listed, attributed or not
//
// A scan the rota could not place has no checkpoint name, and it is listed anyway — labelled by the
// projection's fallback rather than hidden. The scan happened; a patrol whose registration vanished
// because a shift was never recorded would reasonably conclude the app had lost it.
//
// # Kind is always KindCheckpoint here, and that is a known gap
//
// `KindBandit` exists in this package and the mock produces it, but nothing on the stream tells us a scan
// was a bandit catch: `qr.scanned` carries the scanner, and the two cases are physically identical —
// someone scans the patrol's code. Classifying the scanner as a bandit needs a role or team lookup we do
// not do yet. Rather than guess — labelling a post visit "Bandit taget" in front of a patrol that was not
// caught would be worse than labelling it plainly, and it would look like data rather than a bug — every
// real scan is reported as a checkpoint scan. **Task 271** closes this.
func (s projectionSource) ByPatrol(patrolID string) []Scan {
	if patrolID == "" {
		return nil
	}

	rows, err := s.p.ByTeam(s.year, patrolID)
	if err != nil {
		if s.report != nil {
			s.report(err)
		}
		return nil
	}

	// First pass: the earliest attributed scan the patrol has at each checkgroup. A `relative` window
	// opens at the patrol's *own* scan at another group (task 265), so the anchor can only come from the
	// same set of rows. Earliest, not latest, because the window opens the first time the patrol reaches
	// the anchoring group — a later re-scan there must not push the window forward.
	anchorByCheckgroup := make(map[string]int64, len(rows))
	for _, r := range rows {
		if r.CheckgroupID == "" {
			continue
		}
		if prev, seen := anchorByCheckgroup[r.CheckgroupID]; !seen || r.Uts < prev {
			anchorByCheckgroup[r.CheckgroupID] = r.Uts
		}
	}

	out := make([]Scan, 0, len(rows))
	for _, r := range rows {
		anchorUts, hasAnchor := anchorByCheckgroup[r.RelativeCheckgroupID]
		out = append(out, Scan{
			// The event carries no scan id, so the projection's key becomes ours. Stable across
			// replays, which matters because the client uses it as a list key.
			ID:           r.QrID + "-" + time.Unix(r.Uts, 0).UTC().Format("20060102150405"),
			Kind:         KindCheckpoint,
			Label:        label(r),
			CheckpointID: r.CheckpointID,
			Lat:          r.Lat,
			Lng:          r.Lng,
			ScannedAt:    time.Unix(r.Uts, 0).UTC(),
			Verdict: verdictFor(
				r.Uts, r.Scheme, r.OpenFromUts, r.OpenUntilUts, r.OpenDurationMinutes, anchorUts, hasAnchor,
			),
		})
	}
	return out
}

// label names the registration for display.
//
// An unattributed scan gets "Registrering" rather than a blank or a fabricated post name. Blank would
// read as a rendering bug; a guessed name would be a lie about where the patrol was. "Registrering" is
// true, and it is the honest way to say "this happened, we cannot say where".
func label(r ProjectedScan) string {
	if r.CheckpointName != "" {
		return r.CheckpointName
	}
	return "Registrering"
}
