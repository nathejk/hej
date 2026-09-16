package scans

import (
	"sort"
	"time"

	"nathejk.dk/internal/users"
)

// NewMockSource returns a Source seeded with a plausible evening of
// registrations for the two patrol-bearing users in the mock user directory.
// Coordinates sit in central Jutland so they land inside the map's default
// view, and one registration deliberately has no position so clients keep
// handling the nullable case.
//
// The scans deliberately span every on-time state the drawer must render (task 270): on time
// (delta 0), late (positive delta), early (negative delta), a `none`-scheme post with no verdict, and a
// bandit catch that never carries one. A verdict here is a literal, not computed — the mock stands in for
// the projection, and the projection is where verdictFor runs.
//
// The fixture keys off users.Mock*PatrolID rather than repeating the ids: a
// drifting literal here would present as "this patrol has no registrations",
// which is indistinguishable from the real empty state.
func NewMockSource() Source {
	evening := func(hour, min int) time.Time {
		return time.Date(2026, time.August, 24, hour, min, 0, 0, time.UTC)
	}

	m := &mockSource{byPatrol: map[string][]Scan{
		users.MockSpejderPatrolID: {
			{ID: "scan-1042-1", Kind: KindCheckpoint, Label: "Post 1 – Silkeborg Sønderskov", CheckpointID: "cp-1", Lat: pos(56.1382), Lng: pos(9.5521), ScannedAt: evening(18, 40), Verdict: &Verdict{OnTime: true, DeltaSeconds: 0}},
			{ID: "scan-1042-2", Kind: KindCheckpoint, Label: "Post 2 – Kløvermarken", CheckpointID: "cp-2", Lat: pos(56.1804), Lng: pos(9.4812), ScannedAt: evening(20, 5), Verdict: &Verdict{OnTime: false, DeltaSeconds: 12 * 60}},
			{ID: "scan-1042-3", Kind: KindCheckpoint, Label: "Post 3 – Ans Bro", CheckpointID: "cp-3", Lat: pos(56.2311), Lng: pos(9.5216), ScannedAt: evening(21, 14), Verdict: &Verdict{OnTime: false, DeltaSeconds: -5 * 60}},
			// Registered by hand at the post, hence no position. A `none`-scheme post: no window, no verdict.
			{ID: "scan-1042-4", Kind: KindCheckpoint, Label: "Post 4 – Gjern Bakker", CheckpointID: "cp-4", ScannedAt: evening(22, 47)},
			// Unattributable: no shift covered the scanner, so there is no checkpoint and no verdict. Listed
			// anyway — the scan happened, and a patrol whose registration vanished would conclude the app
			// had lost it. Labelled by the projection's honest fallback rather than a guessed post name.
			{ID: "scan-1042-6", Kind: KindCheckpoint, Label: "Registrering", Lat: pos(56.2010), Lng: pos(9.4500), ScannedAt: evening(23, 5)},
			// Bandit catches never carry a verdict.
			{ID: "scan-1042-5", Kind: KindBandit, Label: "Bandit: Sorte Sofie", Lat: pos(56.2609), Lng: pos(9.4103), ScannedAt: evening(23, 32)},
		},
		users.MockBanditPatrolID: {
			{ID: "scan-9001-1", Kind: KindCheckpoint, Label: "Post 2 – Kløvermarken", CheckpointID: "cp-2", Lat: pos(56.1804), Lng: pos(9.4812), ScannedAt: evening(19, 22), Verdict: &Verdict{OnTime: true, DeltaSeconds: 0}},
			{ID: "scan-9001-2", Kind: KindBandit, Label: "Bandit: Grå Greve", Lat: pos(56.0748), Lng: pos(9.3364), ScannedAt: evening(21, 58)},
		},
	}}

	// Sort here so the fixture may be written in reading order while the
	// interface's newest-first promise still holds.
	for _, list := range m.byPatrol {
		sort.Slice(list, func(i, j int) bool { return list[i].ScannedAt.After(list[j].ScannedAt) })
	}
	return m
}

// pos is a helper for the nullable coordinate fields.
func pos(v float64) *float64 { return &v }

type mockSource struct {
	byPatrol map[string][]Scan
}

func (m *mockSource) ByPatrol(patrolID string) []Scan {
	if patrolID == "" {
		return nil
	}
	// Copy so a caller cannot mutate the fixture through the returned slice.
	list := m.byPatrol[patrolID]
	out := make([]Scan, len(list))
	copy(out, list)
	return out
}
