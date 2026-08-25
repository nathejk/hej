package users

// Seeded patrol identities. They are exported because the dev fixtures in other
// packages (e.g. the mock scan source) are scoped to the same patrols, and a
// silent mismatch would look like "this patrol has no registrations" rather
// than a wiring bug.
const (
	MockSpejderPatrolID = "mock-patrol-1042"
	MockBanditPatrolID  = "mock-patrol-9001"
)

// NewMockDirectory returns a Directory backed by an in-code phone → role map,
// seeded with one entry per app role. Intended for dev/testing only; PRD 006's
// person projection replaces it behind the same interface.
//
// Only the two patrol-bearing roles get a patrol; the personnel roles are
// seeded without one on purpose, so the "no patrol" path stays exercised.
//
// Phone numbers must be normalized (see internal/phone) before lookup.
func NewMockDirectory() Directory {
	entries := map[string]User{
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

	// Second index so a session (which carries only the user id) resolves
	// without the phone number it was created from.
	byID := make(map[string]User, len(entries))
	for _, u := range entries {
		byID[u.ID] = u
	}

	return &mockDirectory{entries: entries, byID: byID}
}

type mockDirectory struct {
	entries map[string]User
	byID    map[string]User
}

func (m *mockDirectory) Lookup(phone string) (User, bool) {
	u, ok := m.entries[phone]
	return u, ok
}

func (m *mockDirectory) Get(id string) (User, bool) {
	u, ok := m.byID[id]
	return u, ok
}
