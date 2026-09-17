package users

import (
	"testing"

	"nathejk.dk/nathejk/table/person"
)

// Moderation authority tests (task 300).
//
// The property under test is not "does `team` return true" — that is one line. It is that the
// answer follows the *current* section assignment, and that everything else fails closed.
// Revocation is the case that motivated reading the database on every request instead of
// putting a claim in the session, so it gets an explicit test.

func TestMayModerateGlimt(t *testing.T) {
	cases := []struct {
		name string
		slug string
		want bool
	}{
		{name: "the team section moderates", slug: "team", want: true},
		{name: "the constant and the literal agree", slug: person.SectionTeam, want: true},

		// Section slugs come from a hand-typed, organizer-editable admin field, so
		// folding has to match what the classification map already does — otherwise a
		// trailing space silently removes someone's moderation.
		{name: "case folded", slug: "Team", want: true},
		{name: "whitespace trimmed", slug: " team ", want: true},
		{name: "shouted", slug: "TEAM", want: true},

		// Every other real section from the 2026 tree. None of them moderate, and the
		// two below are the ones most likely to be assumed to.
		{name: "hq does not moderate", slug: "hq", want: false},
		{name: "pr does not moderate", slug: "pr", want: false},
		{name: "postmandskab does not moderate", slug: "postmandskab", want: false},
		{name: "kitchen does not moderate", slug: "koekken", want: false},

		// Fail-closed inputs. An empty slug is what a missing person row, a database
		// outage and an unassigned member all produce.
		{name: "empty slug", slug: "", want: false},
		{name: "whitespace only", slug: "   ", want: false},
		{name: "unknown slug", slug: "teamet", want: false},
		{name: "near miss", slug: "team2", want: false},
		{name: "not the hold column", slug: "teamNumber", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MayModerateGlimt(tc.slug); got != tc.want {
				t.Errorf("MayModerateGlimt(%q) = %v, want %v", tc.slug, got, tc.want)
			}
		})
	}
}

// TestMayModerateGlimtIsNotARole guards the mistake this design exists to prevent.
//
// Every Team-section member has app role `crew`, and so do the kitchen, PR and the
// unclassified fallback. If moderation is ever re-derived from a Role, it goes to all of
// them at once — so assert that no role on its own implies moderation.
func TestMayModerateGlimtIsNotARole(t *testing.T) {
	for _, r := range AllRoles {
		if MayModerateGlimt(string(r)) {
			t.Errorf("role %q used as a section slug granted moderation", r)
		}
	}
}

// TestMayModerateGlimtFollowsTheCurrentAssignment is the revocation property in miniature.
//
// The function is pure, so what it can prove is that the answer tracks its input rather than
// anything cached: the same person, re-asked with a new slug, gets a new answer. The
// end-to-end version — that the *handler* re-reads the slug rather than trusting a session
// claim — lives in cmd/api (see glimt_test.go).
func TestMayModerateGlimtFollowsTheCurrentAssignment(t *testing.T) {
	assigned := person.SectionTeam
	if !MayModerateGlimt(assigned) {
		t.Fatal("newly assigned Team member does not moderate")
	}
	revoked := "koekken"
	if MayModerateGlimt(revoked) {
		t.Error("moderation survived reassignment out of the Team section")
	}
}
