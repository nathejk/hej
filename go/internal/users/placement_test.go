package users

import (
	"reflect"
	"testing"
)

// TestPopulationsOf_ByRole pins the straightforward cases: a participant is listed in
// their own population and nowhere else.
func TestPopulationsOf_ByRole(t *testing.T) {
	tests := []struct {
		name string
		user User
		want []Population
	}{
		{"spejder", User{Role: RoleSpejder}, []Population{PopulationSpejder}},
		{"bandit", User{Role: RoleBandit}, []Population{PopulationBandit}},
		{"gøgler", User{Role: RoleGoegler}, []Population{PopulationGoegler}},
		{"plain crew", User{Role: RoleCrew, SectionSlug: "hq"}, []Population{PopulationCrew}},
		{"samarit", User{Role: RoleSamarit, SectionSlug: "samarit"}, []Population{PopulationCrew}},
		{"crew, no section at all", User{Role: RoleCrew}, []Population{PopulationCrew}},
		{"unknown role", User{Role: Role("bandido")}, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := PopulationsOf(tc.user); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("PopulationsOf() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPopulationsOf_CrewBandit is the case the whole file exists for.
func TestPopulationsOf_CrewBandit(t *testing.T) {
	u := User{Role: RoleCrew, SectionSlug: "bandit", PatrolName: "Klan Ravn"}

	// Listed with the banditter…
	if !IsListedIn(u, PopulationBandit) {
		t.Error("a crew member with section slug bandit is not listed among banditter")
	}
	// …and still among crew. Both, not either.
	if !IsListedIn(u, PopulationCrew) {
		t.Error("a crew bandit vanished from the crew listing; they must appear in both")
	}
	if IsListedIn(u, PopulationSpejder) || IsListedIn(u, PopulationGoegler) {
		t.Error("a crew bandit leaked into an unrelated population")
	}

	// The role must not have moved. This is the regression the placement map is most
	// likely to invite: making slug `bandit` mean RoleBandit would hand this person the
	// bandit's view and take away their patrol lookup.
	if u.Role != RoleCrew {
		t.Fatal("placement changed the role")
	}
	if !MayLookUpPatrol(u.Role) {
		t.Error("a crew bandit lost the patrol lookup — placement must not affect permission")
	}
	if !MayList(u.Role, PopulationGoegler) {
		t.Error("a crew bandit lost the crew view of the directory")
	}
}

// TestPopulationsOf_CrewGoegler covers "including section gøglere" from PRD 007 §6.
func TestPopulationsOf_CrewGoegler(t *testing.T) {
	u := User{Role: RoleCrew, SectionSlug: "goeglerledelse"}

	if !IsListedIn(u, PopulationGoegler) {
		t.Error("goeglerledelse is not listed among gøglere")
	}
	if !IsListedIn(u, PopulationCrew) {
		t.Error("goeglerledelse vanished from the crew listing")
	}
	if u.Role != RoleCrew {
		t.Error("placement changed the role")
	}
}

// TestPopulationsOf_SlugNormalization mirrors classify.go's treatment, so the two maps
// over the same slugs cannot disagree about case or whitespace.
func TestPopulationsOf_SlugNormalization(t *testing.T) {
	for _, slug := range []string{"Bandit", " bandit ", "BANDIT", "\tBandit\n"} {
		u := User{Role: RoleCrew, SectionSlug: slug}
		if !IsListedIn(u, PopulationBandit) {
			t.Errorf("slug %q did not normalize to the bandit population", slug)
		}
	}
}

// TestPopulationsOf_OnlySpecialSlugsMove guards against the map growing by accident: an
// ordinary crew section must stay crew, because its default is correct rather than a
// fallback.
func TestPopulationsOf_OnlySpecialSlugsMove(t *testing.T) {
	ordinary := []string{"hq", "koekken", "rover", "hoensegaard", "pr", "team", "noedtelefon", "postmandskab", "guide", "samarit", "totally-unknown-slug"}

	for _, slug := range ordinary {
		u := User{Role: RoleCrew, SectionSlug: slug}
		got := PopulationsOf(u)
		if !reflect.DeepEqual(got, []Population{PopulationCrew}) {
			t.Errorf("slug %q placed as %v, want crew only", slug, got)
		}
	}
}

// TestPopulationsOf_ParticipantSlugIsIgnored: a section slug on a non-crew role must not
// move them. Only crew are placed by slug.
func TestPopulationsOf_ParticipantSlugIsIgnored(t *testing.T) {
	u := User{Role: RoleSpejder, SectionSlug: "bandit"}
	if !reflect.DeepEqual(PopulationsOf(u), []Population{PopulationSpejder}) {
		t.Error("a spejder with a bandit section slug was moved out of the spejder population")
	}
}
