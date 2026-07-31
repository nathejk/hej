package users

// NewMockDirectory returns a Directory backed by an in-code phone → role map,
// seeded with one entry per app role (spejder, bandit, postmandskab, guide,
// samarit). Intended for dev/testing only; the real Nathejk-records lookup
// will replace this behind the same interface.
//
// Phone numbers must be normalized (see internal/phone) before lookup.
func NewMockDirectory() Directory {
	return &mockDirectory{
		entries: map[string]User{
			"+4530000001": {ID: "mock-spejder-1", Role: RoleSpejder},
			"+4530000002": {ID: "mock-bandit-1", Role: RoleBandit},
			"+4530000003": {ID: "mock-postmandskab-1", Role: RolePostmandskab},
			"+4530000004": {ID: "mock-guide-1", Role: RoleGuide},
			"+4530000005": {ID: "mock-samarit-1", Role: RoleSamarit},
		},
	}
}

type mockDirectory struct {
	entries map[string]User
}

func (m *mockDirectory) Lookup(phone string) (User, bool) {
	u, ok := m.entries[phone]
	return u, ok
}
