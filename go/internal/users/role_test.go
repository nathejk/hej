package users

import "testing"

// The enum is expressed twice — here and in vue/src/stores/session.store.ts. This
// test cannot check the TypeScript side, so it at least pins the Go side against
// accidental edits: a value change here is a wire-format change, since these strings
// travel in GET /api/me and are compared in the frontend router guard.
func TestRoleValuesAreStable(t *testing.T) {
	want := map[Role]string{
		RoleSpejder:      "spejder",
		RoleBandit:       "bandit",
		RolePostmandskab: "postmandskab",
		RoleGuide:        "guide",
		RoleSamarit:      "samarit",
		RoleGoegler:      "gøgler",
		RoleCrew:         "crew",
	}
	for role, s := range want {
		if string(role) != s {
			t.Errorf("role value changed: want %q, got %q", s, string(role))
		}
	}
	if len(AllRoles) != len(want) {
		t.Fatalf("AllRoles has %d entries, want %d — a new role must be added to AllRoles too", len(AllRoles), len(want))
	}
}

func TestAllRolesAreValidAndUnique(t *testing.T) {
	seen := make(map[Role]bool, len(AllRoles))
	for _, r := range AllRoles {
		if !r.Valid() {
			t.Errorf("%q is in AllRoles but Valid() says otherwise", r)
		}
		if seen[r] {
			t.Errorf("%q appears twice in AllRoles", r)
		}
		seen[r] = true
	}
}

func TestUnknownRoleIsInvalid(t *testing.T) {
	for _, r := range []Role{"", "organizer", "Spejder", "gogler"} {
		if Role(r).Valid() {
			t.Errorf("%q should not be a valid role", r)
		}
	}
}

// IsCrew includes the unclassified fallback, because it is a crew member — but
// callers must not use it for access decisions. Pinned so the set does not quietly
// grow to mean "may see crew things".
func TestIsCrew(t *testing.T) {
	crew := map[Role]bool{
		RolePostmandskab: true,
		RoleGuide:        true,
		RoleSamarit:      true,
		RoleCrew:         true,
		RoleSpejder:      false,
		RoleBandit:       false,
		RoleGoegler:      false,
	}
	for role, want := range crew {
		if got := role.IsCrew(); got != want {
			t.Errorf("%q.IsCrew() = %v, want %v", role, got, want)
		}
	}
}

// Every role should be reachable in dev, or a role-gated page can only be tested in
// production. This is what caught that gøgler and crew had no seed entry.
func TestMockDirectoryCoversEveryRole(t *testing.T) {
	dir := NewMockDirectory()

	found := make(map[Role]bool)
	// The mock is keyed by normalized phone; walk the seeded range rather than
	// reaching into its internals.
	for _, phone := range []string{
		"+4530000001", "+4530000002", "+4530000003", "+4530000004",
		"+4530000005", "+4530000006", "+4530000007",
	} {
		u, ok := dir.Lookup(phone)
		if !ok {
			t.Errorf("mock directory has no entry for %s", phone)
			continue
		}
		found[u.Role] = true
	}

	for _, r := range AllRoles {
		if !found[r] {
			t.Errorf("no mock directory entry for role %q — add one so its pages are testable in dev", r)
		}
	}
}
