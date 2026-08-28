package users

// Seeded patrol identities. They are exported because the dev fixtures in other
// packages (e.g. the mock scan source) are scoped to the same patrols, and a
// silent mismatch would look like "this patrol has no registrations" rather
// than a wiring bug.
const (
	MockSpejderPatrolID = "mock-patrol-1042"
	MockBanditPatrolID  = "mock-patrol-9001"
)

// MockSharedPhone is seeded with TWO people in the same patrol, so the
// phone-collision path (siblings sharing one phone) is exercised in dev rather than
// first appearing in production. Their patrol is identical on purpose: it is the case
// where the team name does **not** disambiguate and only the first name does.
//
// See the Directory doc comment and tasks 071/079.
const MockSharedPhone = "+4530000008"

// MockSharedCrewPhone is seeded with two crew members in **different sections**, the
// case where the section is the only thing that tells the candidates apart. Without a
// fixture like this, the crew branch of the chooser would first be exercised on real
// people.
const MockSharedCrewPhone = "+4530000009"

// NewMockDirectory returns a Directory backed by an in-code phone → role map,
// seeded with one entry per app role plus one shared number. Intended for
// dev/testing only; PRD 006's person projection replaces it behind the same
// interface.
//
// Only the two patrol-bearing roles get a patrol; the personnel roles are
// seeded without one on purpose, so the "no patrol" path stays exercised.
//
// The same applies to the guardian number (task 093): spejder get one, everybody
// else is seeded with nil, and one spejder is seeded with a pointer to "" — the
// "expected but missing" case the profile page renders as "Ikke registreret".
// Without that fixture the distinction would first be exercised on real data.
//
// Phone numbers must be normalized (see internal/phone) before lookup.
func NewMockDirectory() Directory {
	parentPhone := "+4520000001"
	noParentOnFile := ""

	single := map[string]User{
		"+4530000001": {
			ID: "mock-spejder-1", Name: "Signe Spejder", Role: RoleSpejder,
			PatrolID: MockSpejderPatrolID, PatrolName: "Patrulje Ravnene",
			Phone: "+4530000001", PhoneParent: &parentPhone,
			Address: "Spejdervej 12", PostalCode: "8600", City: "Silkeborg",
		},
		"+4530000002": {
			ID: "mock-bandit-1", Name: "Bo Bandit", Role: RoleBandit,
			PatrolID: MockBanditPatrolID, PatrolName: "Banditgruppe Nord",
			Phone:   "+4530000002",
			Address: "Klanvej 3", PostalCode: "8000", City: "Aarhus C",
		},
		"+4530000003": {ID: "mock-postmandskab-1", Name: "Mads Post", Role: RolePostmandskab, Section: "Postmandskab", Phone: "+4530000003"},
		"+4530000004": {ID: "mock-guide-1", Name: "Gitte Guide", Role: RoleGuide, Section: "Guider", Phone: "+4530000004"},
		"+4530000005": {ID: "mock-samarit-1", Name: "Sara Samarit", Role: RoleSamarit, Section: "Samaritter", Phone: "+4530000005"},
		"+4530000006": {ID: "mock-goegler-1", Name: "Gørn Gøgler", Role: RoleGoegler, Phone: "+4530000006"},
		// Unclassified crew: seeded so the least-privileged fallback path is
		// exercised in dev rather than only appearing in production the first time an
		// organizer renames a section. Its section label is deliberately one the
		// classifier does not recognise.
		"+4530000007": {ID: "mock-crew-1", Name: "Kim Krew", Role: RoleCrew, Section: "Kagebord", Phone: "+4530000007"},
	}

	entries := make(map[string][]User, len(single)+1)
	for p, u := range single {
		entries[p] = []User{u}
	}
	// Two siblings on one phone, same patrol — only the first name distinguishes them.
	// Villads carries an empty-but-present guardian number, i.e. the "missing" state.
	entries[MockSharedPhone] = []User{
		{
			ID: "mock-sibling-a", Name: "Freja Mikkelsen", Role: RoleSpejder,
			PatrolID: MockSpejderPatrolID, PatrolName: "Patrulje Ravnene",
			Phone: MockSharedPhone, PhoneParent: &parentPhone,
			Address: "Mosevej 7", PostalCode: "8600", City: "Silkeborg",
		},
		{
			ID: "mock-sibling-b", Name: "Villads Mikkelsen", Role: RoleSpejder,
			PatrolID: MockSpejderPatrolID, PatrolName: "Patrulje Ravnene",
			Phone: MockSharedPhone, PhoneParent: &noParentOnFile,
			Address: "Mosevej 7", PostalCode: "8600", City: "Silkeborg",
		},
	}
	// Two crew on one phone in different sections — here the section is what tells
	// them apart, and a chooser showing only names would leave the user guessing.
	entries[MockSharedCrewPhone] = []User{
		{ID: "mock-crew-samarit", Name: "Anne Jensen", Role: RoleSamarit, Section: "Samaritter", Phone: MockSharedCrewPhone},
		{ID: "mock-crew-guide", Name: "Anne Jensen", Role: RoleGuide, Section: "Guider", Phone: MockSharedCrewPhone},
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
