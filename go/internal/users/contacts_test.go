package users

import "testing"

// The tests below are exhaustive on purpose. PRD 007 treats the spejder/bandit
// separation as a correctness requirement rather than a preference — a leak damages the
// event, not just a person's privacy — so every viewer/subject pair is asserted
// explicitly instead of relying on the implementation's shape.

// TestMayUseContacts_ExcludesOnlySpejder pins the pane's audience.
func TestMayUseContacts_ExcludesOnlySpejder(t *testing.T) {
	for _, r := range AllRoles {
		want := r != RoleSpejder
		if got := MayUseContacts(r); got != want {
			t.Errorf("MayUseContacts(%q) = %v, want %v", r, got, want)
		}
	}
}

// TestMayUseContacts_RejectsUnknownRole guards the localStorage/session path: a role
// that is not in AllRoles must not be treated as "some kind of crew".
func TestMayUseContacts_RejectsUnknownRole(t *testing.T) {
	if MayUseContacts(Role("bandido")) {
		t.Error("an unknown role was granted the contacts pane")
	}
	if MayLookUpPatrol(Role("")) {
		t.Error("the empty role was granted the patrol lookup")
	}
}

// TestMayList_EveryPair is the access matrix, written out.
func TestMayList_EveryPair(t *testing.T) {
	// want[viewer][subject]
	want := map[Role]map[Population]bool{
		RoleSpejder: {
			PopulationSpejder: false,
			PopulationBandit:  false,
			PopulationGoegler: false,
			PopulationCrew:    false,
		},
		RoleBandit: {
			PopulationSpejder: false,
			PopulationBandit:  true,
			PopulationGoegler: false,
			PopulationCrew:    true,
		},
		RoleGoegler: {
			PopulationSpejder: false,
			PopulationBandit:  false,
			PopulationGoegler: true,
			PopulationCrew:    true,
		},
		RoleSamarit: {
			PopulationSpejder: false,
			PopulationBandit:  true,
			PopulationGoegler: true,
			PopulationCrew:    true,
		},
		RoleGuide: {
			PopulationSpejder: false,
			PopulationBandit:  true,
			PopulationGoegler: true,
			PopulationCrew:    true,
		},
		RolePostmandskab: {
			PopulationSpejder: false,
			PopulationBandit:  true,
			PopulationGoegler: true,
			PopulationCrew:    true,
		},
		RoleCrew: {
			PopulationSpejder: false,
			PopulationBandit:  true,
			PopulationGoegler: true,
			PopulationCrew:    true,
		},
	}

	// Fail loudly if a role is added without a row here, rather than silently not
	// testing it — the mistake this whole file exists to prevent.
	if len(want) != len(AllRoles) {
		t.Fatalf("matrix covers %d roles, AllRoles has %d — add the new role to this test", len(want), len(AllRoles))
	}

	for _, viewer := range AllRoles {
		row, ok := want[viewer]
		if !ok {
			t.Fatalf("no expectations for role %q", viewer)
		}
		if len(row) != len(AllPopulations) {
			t.Fatalf("role %q covers %d populations, AllPopulations has %d", viewer, len(row), len(AllPopulations))
		}
		for _, subject := range AllPopulations {
			if got := MayList(viewer, subject); got != row[subject] {
				t.Errorf("MayList(%q, %q) = %v, want %v", viewer, subject, got, row[subject])
			}
		}
	}
}

// TestMayList_NobodyListsSpejdere states the property directly, so it survives a
// rewrite of the matrix above.
func TestMayList_NobodyListsSpejdere(t *testing.T) {
	for _, viewer := range AllRoles {
		if MayList(viewer, PopulationSpejder) {
			t.Errorf("role %q may list spejdere; no role may", viewer)
		}
	}
}

// TestMayList_SpejderBanditIsSymmetric covers the game-integrity property in both
// directions. It is implied by the matrix, but asserted separately because "symmetric"
// is the part a future change is most likely to break on one side only.
func TestMayList_SpejderBanditIsSymmetric(t *testing.T) {
	if MayList(RoleSpejder, PopulationBandit) {
		t.Error("a spejder may list banditter")
	}
	if MayList(RoleBandit, PopulationSpejder) {
		t.Error("a bandit may list spejdere")
	}
}

// TestMayList_ParticipantsAreSeparated pins the other half of the game rule: banditter
// and gøglere cannot see each other either.
func TestMayList_ParticipantsAreSeparated(t *testing.T) {
	if MayList(RoleBandit, PopulationGoegler) {
		t.Error("a bandit may list gøglere")
	}
	if MayList(RoleGoegler, PopulationBandit) {
		t.Error("a gøgler may list banditter")
	}
}

// TestMayLookUpPatrol_CrewOnly pins the lookup's audience, including the fallback role.
func TestMayLookUpPatrol_CrewOnly(t *testing.T) {
	for _, r := range AllRoles {
		want := r.IsCrew()
		if got := MayLookUpPatrol(r); got != want {
			t.Errorf("MayLookUpPatrol(%q) = %v, want %v", r, got, want)
		}
	}

	// Stated explicitly: the unclassified fallback gets the lookup. This is a decision
	// (2026-08-31), not an oversight — in the real data every crew member currently
	// lands on RoleCrew (task 078), so excluding it would ship a feature with no users.
	if !MayLookUpPatrol(RoleCrew) {
		t.Error("RoleCrew must have the patrol lookup; see PRD 007 §11.3")
	}
}

// TestCrewMayLookUpButNotList is the distinction the two functions exist to keep apart.
func TestCrewMayLookUpButNotList(t *testing.T) {
	for _, r := range AllRoles {
		if !r.IsCrew() {
			continue
		}
		if MayList(r, PopulationSpejder) {
			t.Errorf("crew role %q may list spejdere; the lookup is the only path", r)
		}
		if !MayLookUpPatrol(r) {
			t.Errorf("crew role %q lost the patrol lookup", r)
		}
	}
}
