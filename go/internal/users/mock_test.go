package users_test

import (
	"testing"

	"nathejk.dk/internal/users"
)

func TestMockDirectory_SeedsAllRoles(t *testing.T) {
	dir := users.NewMockDirectory()

	seeded := map[string]users.Role{
		"+4530000001": users.RoleSpejder,
		"+4530000002": users.RoleBandit,
		"+4530000003": users.RolePostmandskab,
		"+4530000004": users.RoleGuide,
		"+4530000005": users.RoleSamarit,
	}

	for phone, wantRole := range seeded {
		u, ok := dir.Lookup(phone)
		if !ok {
			t.Errorf("%s: expected recognized", phone)
			continue
		}
		if u.Role != wantRole {
			t.Errorf("%s: role = %q, want %q", phone, u.Role, wantRole)
		}
		if u.ID == "" {
			t.Errorf("%s: user id must not be empty", phone)
		}
	}
}

func TestMockDirectory_UnknownPhoneNotRecognized(t *testing.T) {
	dir := users.NewMockDirectory()
	if u, ok := dir.Lookup("+4599999999"); ok {
		t.Errorf("unknown phone should not be recognized, got %+v", u)
	}
}
