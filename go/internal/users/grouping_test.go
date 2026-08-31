package users

import "testing"

func TestGroupPathFor_BanditsGroupedByKlan(t *testing.T) {
	viewer := User{Role: RoleBandit, PatrolID: "klan-1", PatrolName: "Klan Ravn"}
	sameKlan := User{Role: RoleBandit, PatrolID: "klan-1", PatrolName: "Klan Ravn"}
	otherKlan := User{Role: RoleBandit, PatrolID: "klan-2", PatrolName: "Klan Ulv"}

	own := GroupPathFor(viewer, sameKlan, PopulationBandit)
	if len(own) != 1 {
		t.Fatalf("want a one-level path, got %v", own)
	}
	if own[0].ID != "klan-1" || own[0].Label != "Klan Ravn" {
		t.Errorf("unexpected group %+v", own[0])
	}
	if !own[0].IsOwn {
		t.Error("the viewer's own klan is not marked as own; it would not expand by default")
	}

	other := GroupPathFor(viewer, otherKlan, PopulationBandit)
	if len(other) != 1 || other[0].ID != "klan-2" {
		t.Fatalf("unexpected group %v", other)
	}
	if other[0].IsOwn {
		t.Error("another klan was marked as the viewer's own")
	}
}

func TestGroupPathFor_BanditWithoutKlan(t *testing.T) {
	viewer := User{Role: RoleBandit, PatrolID: "klan-1"}
	orphan := User{Role: RoleBandit}

	got := GroupPathFor(viewer, orphan, PopulationBandit)
	if len(got) != 1 {
		t.Fatalf("a bandit with no klan must still be listed, got %v", got)
	}
	if got[0].Label != "Uden klan" {
		t.Errorf("unexpected label %q", got[0].Label)
	}

	// Two people both missing data must not be treated as sharing a group.
	viewerNoKlan := User{Role: RoleBandit}
	got = GroupPathFor(viewerNoKlan, orphan, PopulationBandit)
	if got[0].IsOwn {
		t.Error("the no-klan group was marked as own; missing data is not a shared klan")
	}
}

func TestGroupPathFor_FlatPopulations(t *testing.T) {
	goegler := User{Role: RoleGoegler}
	crew := User{Role: RoleCrew, SectionSlug: "hq"}

	// A gøgler viewing gøglere: own group, expands by default.
	got := GroupPathFor(goegler, goegler, PopulationGoegler)
	if len(got) != 1 || got[0].Label != "Gøglere" || !got[0].IsOwn {
		t.Errorf("unexpected gøgler group %+v", got)
	}

	// Crew viewing gøglere: permitted, but not their own group.
	got = GroupPathFor(crew, goegler, PopulationGoegler)
	if len(got) != 1 || got[0].IsOwn {
		t.Errorf("the gøgler list should not auto-expand for crew: %+v", got)
	}

	// Crew viewing crew: own group.
	got = GroupPathFor(crew, crew, PopulationCrew)
	if len(got) != 1 || got[0].Label != "Crew" || !got[0].IsOwn {
		t.Errorf("unexpected crew group %+v", got)
	}

	// A bandit viewing crew: permitted, not own.
	got = GroupPathFor(User{Role: RoleBandit}, crew, PopulationCrew)
	if len(got) != 1 || got[0].IsOwn {
		t.Errorf("the crew list should not auto-expand for a bandit: %+v", got)
	}
}

// TestGroupPathFor_CrewBanditIsGroupedPerViewer is the payoff of grouping being
// per-viewer: the same person is a klan member to a bandit and crew to a crew member.
func TestGroupPathFor_CrewBanditIsGroupedPerViewer(t *testing.T) {
	crewBandit := User{Role: RoleCrew, SectionSlug: "bandit", PatrolID: "klan-1", PatrolName: "Klan Ravn"}

	banditViewer := User{Role: RoleBandit, PatrolID: "klan-1", PatrolName: "Klan Ravn"}
	got := GroupPathFor(banditViewer, crewBandit, PopulationBandit)
	if len(got) != 1 || got[0].ID != "klan-1" || !got[0].IsOwn {
		t.Errorf("a bandit should see a crew bandit in their shared klan: %+v", got)
	}

	crewViewer := User{Role: RoleSamarit, SectionSlug: "samarit"}
	got = GroupPathFor(crewViewer, crewBandit, PopulationCrew)
	if len(got) != 1 || got[0].ID != groupIDCrew {
		t.Errorf("crew should see a crew bandit among crew: %+v", got)
	}
}

// TestGroupPathFor_RefusesWithoutPermission: grouping must not be a way around
// authorization, so a caller that forgets to check still gets nothing renderable.
func TestGroupPathFor_RefusesWithoutPermission(t *testing.T) {
	bandit := User{Role: RoleBandit, PatrolID: "klan-1"}
	goegler := User{Role: RoleGoegler}
	spejder := User{Role: RoleSpejder, PatrolID: "patrol-138", PatrolName: "Patrulje 138"}

	if got := GroupPathFor(bandit, goegler, PopulationGoegler); got != nil {
		t.Errorf("a bandit got a gøgler group: %v", got)
	}
	if got := GroupPathFor(spejder, bandit, PopulationBandit); got != nil {
		t.Errorf("a spejder got a group at all: %v", got)
	}
	for _, viewer := range AllRoles {
		if got := GroupPathFor(User{Role: viewer}, spejder, PopulationSpejder); got != nil {
			t.Errorf("role %q got a spejder group: %v", viewer, got)
		}
	}
}

// TestGroupPathFor_RefusesMismatchedPopulation: asking for a population the subject is
// not in must not fabricate a group.
func TestGroupPathFor_RefusesMismatchedPopulation(t *testing.T) {
	crew := User{Role: RoleCrew, SectionSlug: "hq"}
	if got := GroupPathFor(User{Role: RoleBandit, PatrolID: "k"}, crew, PopulationBandit); got != nil {
		t.Errorf("a plain crew member was grouped as a bandit: %v", got)
	}
}
