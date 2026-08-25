package users

// Seeded patrol identities. They are exported because the dev fixtures in other
// packages (e.g. the mock scan source) are scoped to the same patrols, and a
// silent mismatch would look like "this patrol has no registrations" rather
// than a wiring bug.
const (
	MockSpejderPatrolID = "mock-patrol-1042"
	MockBanditPatrolID  = "mock-patrol-9001"
)

// MockSharedPhone is seeded with TWO people, so the phone-collision path (siblings
// sharing one phone) is exercised in dev rather than first appearing in production.
// See the Directory doc comment and task 071.
const MockSharedPhone = "+4530000008"

// NewMockDirectory returns a Directory backed by an in-code phone → role map,
// seeded with one entry per app role plus one shared number. Intended for
// dev/testing only; PRD 006's person projection replaces it behind the same
// interface.
//
// Only the two patrol-bearing roles get a patrol; the personnel roles are
// seeded without one on purpose, so the "no patrol" path stays exercised.
//
// Phone numbers must be normalized (see internal/phone) before lookup.
func NewMockDirectory() Directory {
	single := map[string]User{
		"+4530000001": {ID: "mock-spejder-1", Role: RoleSpejder, PatrolID: MockSpejderPatrolID, PatrolName: "Patrulje Ravnene"},
		"+4530000002": {ID: "mock-bandit-1", Role: RoleBandit, PatrolID: MockBanditPatrolID, PatrolName: "Banditgruppe Nord"},
		"+4530000003": {ID: "mock-postmandskab-1", Role: RolePostmandskab},
		"+4530000004": {ID: "mock-guide-1", Role: RoleGuide},
		"+4530000005": {ID: "mock-samarit-1", Role: RoleSamarit},
		"+4530000006": {ID: "mock-goegler-1", Role: RoleGoegler},
		// Unclassified crew: seeded so the least-privileged fallback path is
		// exercised in dev rather than only appearing in production the first time an
		// organizer renames a section.
		"+4530000007": {ID: "mock-crew-1", Role: RoleCrew},
	}

	entries := make(map[string][]User, len(single)+1)
	for p, u := range single {
		entries[p] = []User{u}
	}
	// Two siblings on one phone.
	entries[MockSharedPhone] = []User{
		{ID: "mock-sibling-a", Role: RoleSpejder, PatrolID: MockSpejderPatrolID, PatrolName: "Patrulje Ravnene"},
		{ID: "mock-sibling-b", Role: RoleSpejder, PatrolID: MockSpejderPatrolID, PatrolName: "Patrulje Ravnene"},
	}

	// Second index so a session (which carries only the user id) resolves
	// without the phone number it was created from.
	byID := make(map[string]User, len(entries))
	for _, users := range entries {
		for _, u := range users {
			byID[u.ID] = u
		}
	}

	return &mockDirectory{entries: entries, byID: byID}
}

type mockDirectory struct {
	entries map[string][]User
	byID    map[string]User
}

func (m *mockDirectory) LookupAll(phone string) []User {
	return m.entries[phone]
}

// Lookup returns the single match, and reports not-found when the number is shared.
// See the Directory doc comment: collapsing "ambiguous" into "not found" is what
// stops a caller logging someone in as the wrong person.
func (m *mockDirectory) Lookup(phone string) (User, bool) {
	matches := m.entries[phone]
	if len(matches) != 1 {
		return User{}, false
	}
	return matches[0], true
}

func (m *mockDirectory) Get(id string) (User, bool) {
	u, ok := m.byID[id]
	return u, ok
}
