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
		"+4530000006": users.RoleGoegler,
		"+4530000007": users.RoleCrew,
	}

	// The name of this test claims "all roles", so hold it to that: a role added to
	// users.AllRoles without a seed entry must fail here rather than leaving the test
	// passing on a stale subset (which is what happened when gøgler and crew were
	// added).
	if len(seeded) != len(users.AllRoles) {
		t.Fatalf("seeded %d roles but users.AllRoles has %d — add the missing seed entries", len(seeded), len(users.AllRoles))
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

func TestMockDirectory_GetByID(t *testing.T) {
	dir := users.NewMockDirectory()

	spejder, ok := dir.Lookup("+4530000001")
	if !ok {
		t.Fatal("expected seeded spejder")
	}
	byID, ok := dir.Get(spejder.ID)
	if !ok {
		t.Fatalf("Get(%q): expected found", spejder.ID)
	}
	if byID != spejder {
		t.Errorf("Get = %+v, want %+v", byID, spejder)
	}
	if _, ok := dir.Get("nope"); ok {
		t.Error("unknown id should not be found")
	}
}

func TestMockDirectory_PatrolOnlyForPatrolRoles(t *testing.T) {
	dir := users.NewMockDirectory()

	withPatrol := map[string]string{
		"+4530000001": users.MockSpejderPatrolID,
		"+4530000002": users.MockBanditPatrolID,
	}
	for phone, wantID := range withPatrol {
		u, _ := dir.Lookup(phone)
		if u.PatrolID != wantID {
			t.Errorf("%s: patrol id = %q, want %q", phone, u.PatrolID, wantID)
		}
		if u.PatrolName == "" {
			t.Errorf("%s: patrol name must not be empty", phone)
		}
	}

	// Personnel roles have no patrol; consumers rely on "" meaning "none".
	for _, phone := range []string{"+4530000003", "+4530000004", "+4530000005"} {
		u, _ := dir.Lookup(phone)
		if u.PatrolID != "" {
			t.Errorf("%s: patrol id = %q, want empty", phone, u.PatrolID)
		}
	}
}
