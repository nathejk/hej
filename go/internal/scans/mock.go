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
// The fixture keys off users.Mock*PatrolID rather than repeating the ids: a
// drifting literal here would present as "this patrol has no registrations",
// which is indistinguishable from the real empty state.
func NewMockSource() Source {
	evening := func(hour, min int) time.Time {
		return time.Date(2026, time.August, 24, hour, min, 0, 0, time.UTC)
	}

	m := &mockSource{byPatrol: map[string][]Scan{
		users.MockSpejderPatrolID: {
			{ID: "scan-1042-1", Kind: KindCheckpoint, Label: "Post 1 – Silkeborg Sønderskov", Lat: pos(56.1382), Lng: pos(9.5521), ScannedAt: evening(18, 40)},
			{ID: "scan-1042-2", Kind: KindCheckpoint, Label: "Post 2 – Kløvermarken", Lat: pos(56.1804), Lng: pos(9.4812), ScannedAt: evening(20, 5)},
			{ID: "scan-1042-3", Kind: KindCheckpoint, Label: "Post 3 – Ans Bro", Lat: pos(56.2311), Lng: pos(9.5216), ScannedAt: evening(21, 14)},
			// Registered by hand at the post, hence no position.
			{ID: "scan-1042-4", Kind: KindCheckpoint, Label: "Post 4 – Gjern Bakker", ScannedAt: evening(22, 47)},
			{ID: "scan-1042-5", Kind: KindBandit, Label: "Bandit: Sorte Sofie", Lat: pos(56.2609), Lng: pos(9.4103), ScannedAt: evening(23, 32)},
		},
		users.MockBanditPatrolID: {
			{ID: "scan-9001-1", Kind: KindCheckpoint, Label: "Post 2 – Kløvermarken", Lat: pos(56.1804), Lng: pos(9.4812), ScannedAt: evening(19, 22)},
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
