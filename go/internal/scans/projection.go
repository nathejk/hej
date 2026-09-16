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

// Banditter reports which of the year's people are banditter, so a scan can be classified.
//
// # Why this seam exists at all
//
// Nothing upstream says a scan was a bandit catch. `qr.scanned` carries the scanner's id, phone and
// position — and no role, no kind, no flag (checked against the contract for task 271). Physically the two
// cases are the same act: someone scans the patrol's code. The only way to tell them apart is to ask who
// the scanner *was*, which is a fact this repo already holds — the person projection classifies a senior as
// `RoleBandit`.
//
// # Why a set rather than a lookup per scan
//
// One read per request instead of one per registration. A patrol's list is short but the map refetches it,
// and the year's banditter are a few dozen rows — so fetching the set once and testing membership is both
// cheaper and simpler than N indexed reads.
//
// Declared here, like Projection, so `internal/scans` keeps owning the shape it needs and depends on
// nothing under nathejk/table. The concrete adapter lives in cmd/api.
type Banditter interface {
	// BanditIDs returns the person ids of the year's banditter. An empty set is a normal answer — before
	// the seniors are classified, and in any year with no banditter.
	BanditIDs(year string) (map[string]bool, error)
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

	// ScannerID is the person who scanned the patrol's code.
	//
	// The only handle on *what kind* of registration this is: the event says nothing about the scanner's
	// role, and a post visit and a bandit catch are physically identical acts (task 271).
	ScannerID string

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
func NewProjectionSource(p Projection, year string, banditter Banditter, report func(error)) Source {
	return projectionSource{p: p, year: year, banditter: banditter, report: report}
}

type projectionSource struct {
	p         Projection
	year      string
	banditter Banditter
	report    func(error)
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
// # How a bandit catch is told apart from a post visit
//
// By asking who scanned. Nothing on the stream distinguishes them — `qr.scanned` carries the scanner's id
// and no role, and the two are the same physical act — so the scanner is resolved against the year's
// banditter (see Banditter). A scanner we cannot classify stays a **checkpoint** scan: labelling a post
// visit "Bandit taget" in front of a patrol that was not caught would be worse than labelling it plainly,
// and the failure would be invisible to us because it looks like data rather than a bug. So every
// uncertainty resolves towards the dull answer (task 271).
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

	// The year's banditter, once. A failure here must not fail the list: the registrations are the point,
	// and the classification is a label on them. Reported, then treated as "no banditter known", which
	// leaves every scan a checkpoint visit — the safe direction.
	banditIDs := map[string]bool{}
	if s.banditter != nil {
		if ids, err := s.banditter.BanditIDs(s.year); err != nil {
			if s.report != nil {
				s.report(err)
			}
		} else {
			banditIDs = ids
		}
	}

	out := make([]Scan, 0, len(rows))
	for _, r := range rows {
		anchorUts, hasAnchor := anchorByCheckgroup[r.RelativeCheckgroupID]
		kind := kindFor(r, banditIDs)
		out = append(out, Scan{
			// The event carries no scan id, so the projection's key becomes ours. Stable across
			// replays, which matters because the client uses it as a list key.
			ID:           r.QrID + "-" + time.Unix(r.Uts, 0).UTC().Format("20060102150405"),
			Kind:         kind,
			Label:        label(r, kind),
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

// kindFor classifies a registration from who scanned it.
//
// A bandit catch only when the scanner is *known* to be a bandit. An empty scanner id, a scanner absent
// from the person projection, a failed lookup and a year with no classified banditter all yield
// KindCheckpoint — because the cost of the two mistakes is not symmetric. Missing a catch understates what
// happened; inventing one tells a patrol they were caught when they were not, which they would act on.
func kindFor(r ProjectedScan, banditIDs map[string]bool) Kind {
	if r.ScannerID != "" && banditIDs[r.ScannerID] {
		return KindBandit
	}
	return KindCheckpoint
}

// label names the registration for display.
//
// An unattributed scan gets "Registrering" rather than a blank or a fabricated post name. Blank would
// read as a rendering bug; a guessed name would be a lie about where the patrol was. "Registrering" is
// true, and it is the honest way to say "this happened, we cannot say where".
//
// A bandit catch with no post name is "Bandit" instead — also true, and more use than "Registrering" next
// to the drawer's skull. We know the scanner was a bandit but not *which* one: the scan event carries an id,
// and naming the individual would tell a patrol who caught them, which is not this app's business.
func label(r ProjectedScan, kind Kind) string {
	if r.CheckpointName != "" {
		return r.CheckpointName
	}
	if kind == KindBandit {
		return "Bandit"
	}
	return "Registrering"
}
