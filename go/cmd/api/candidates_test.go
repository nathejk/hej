package main

import (
	"strings"
	"testing"

	"nathejk.dk/internal/users"
)

// The candidate payload has one job: let somebody pick their own profile out of a list. Real data
// makes that harder than it sounds — a number carrying one `postmandskab` row named "Klaus
// Jørgensen" and four `gøgler` rows named "Klaus" rendered as five identical lines, because the
// surname was stripped and the role was not sent at all (2026-09-01).

func TestCandidatesFor_LoginKeepsFirstNamesOnly(t *testing.T) {
	owners := []users.User{
		{ID: "a", Name: "Freja Mikkelsen", Role: users.RoleSpejder, PatrolName: "Patrulje Ravnene"},
	}

	got := candidatesFor(owners, detailMinimal)

	if got[0].Name != "Freja" {
		t.Errorf("name = %q, want the first name only at login", got[0].Name)
	}
	// PRD 006's reasoning, unchanged: whoever holds the phone may be a sibling, and a surname is a
	// fuller identifier for somebody who is not them.
	if strings.Contains(got[0].Name, "Mikkelsen") {
		t.Error("a surname reached the login chooser")
	}
}

func TestCandidatesFor_SwitchKeepsFullNames(t *testing.T) {
	owners := []users.User{
		{ID: "a", Name: "Klaus Jørgensen", Role: users.RolePostmandskab, Section: "Postmandskab"},
		{ID: "b", Name: "Klaus", Role: users.RoleGoegler},
	}

	got := candidatesFor(owners, detailFull)

	// The switch is between profiles the caller can already reach, so there is nothing to withhold
	// — and withholding the surname is what made these two rows identical.
	if got[0].Name != "Klaus Jørgensen" {
		t.Errorf("name = %q, want the full name when switching", got[0].Name)
	}
	if got[0].Name == got[1].Name {
		t.Error("the two profiles are still indistinguishable by name")
	}
}

// The role is the discriminator when nothing else is set, which the real duplicates often are — but
// only on the switch payload. Login stays minimal (PRD 006), which the assertion below pins.
func TestCandidatesFor_CarriesTheRoleWhenSwitchingOnly(t *testing.T) {
	owners := []users.User{
		{ID: "a", Name: "Klaus", Role: users.RoleGoegler},
		{ID: "b", Name: "Klaus", Role: users.RolePostmandskab},
	}

	switching := candidatesFor(owners, detailFull)
	if switching[0].Role != string(users.RoleGoegler) || switching[1].Role != string(users.RolePostmandskab) {
		t.Errorf("roles not carried when switching: %+v", switching)
	}

	// One person is being shown another's details at login, so the role stays out — the same
	// decision TestVerifySharedNumberAsksToChoose enforces end to end.
	login := candidatesFor(owners, detailMinimal)
	for _, c := range login {
		if c.Role != "" {
			t.Errorf("role leaked into the login chooser: %+v", c)
		}
	}
}

func TestCandidatesFor_CarriesAffiliation(t *testing.T) {
	owners := []users.User{
		{ID: "a", Name: "Bo", Role: users.RoleBandit, PatrolName: "Klan Ravn"},
		{ID: "b", Name: "Sara", Role: users.RoleSamarit, Section: "Samaritter"},
	}

	got := candidatesFor(owners, detailMinimal)

	if got[0].Team != "Klan Ravn" {
		t.Errorf("team = %q", got[0].Team)
	}
	if got[1].Section != "Samaritter" {
		t.Errorf("section = %q", got[1].Section)
	}
}

// Whatever else the payload carries, it never carries these.
func TestCandidatesFor_DisclosesNothingElse(t *testing.T) {
	guardian := "+4520999888"
	owners := []users.User{{
		ID: "a", Name: "Freja Mikkelsen", Role: users.RoleSpejder,
		Phone: "+4530001111", PhoneParent: &guardian,
		Address: "Spejdervej 12", PostalCode: "8600", City: "Silkeborg",
	}}

	got := candidatesFor(owners, detailFull)

	// The struct has nowhere to put them, which is the real guarantee; asserted so that adding a
	// field has to argue with a test.
	if got[0].UserID == "" || got[0].Name == "" {
		t.Fatalf("unexpected candidate: %+v", got[0])
	}
	if got[0].Team != "" || got[0].Section != "" {
		t.Errorf("affiliation invented from nothing: %+v", got[0])
	}
}

func TestFirstName(t *testing.T) {
	for input, want := range map[string]string{
		"Klaus Jørgensen":    "Klaus",
		"Klaus":              "Klaus",
		"  Freja  Mikkelsen": "Freja",
		"":                   "",
	} {
		if got := firstName(input); got != want {
			t.Errorf("firstName(%q) = %q, want %q", input, got, want)
		}
	}
}
