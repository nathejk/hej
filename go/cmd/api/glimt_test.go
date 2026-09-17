package main

import (
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/nathejk/table/person"
)

// isGlimtModerator tests (task 300).
//
// The unit test in internal/users proves the predicate. What matters here is the wiring
// around it: that the app asks the projection *every time*, so that revoking the Team
// assignment revokes the power — the reason this is a database read and not a session claim.

func TestIsGlimtModerator(t *testing.T) {
	people := &stubPeople{found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)

	people.p = person.Person{PersonID: "member-1", SectionSlug: person.SectionTeam}
	if !app.isGlimtModerator("member-1") {
		t.Fatal("a member of the Team section does not moderate")
	}

	people.p = person.Person{PersonID: "member-1", SectionSlug: "koekken"}
	if app.isGlimtModerator("member-1") {
		t.Error("moderation survived reassignment out of the Team section")
	}
}

// TestIsGlimtModeratorAsksEveryTime is the test that would fail if someone "optimised" this
// into a per-session or per-connection cache without solving invalidation.
func TestIsGlimtModeratorAsksEveryTime(t *testing.T) {
	people := &stubPeople{found: true, p: person.Person{SectionSlug: person.SectionTeam}}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)

	for i := 0; i < 3; i++ {
		app.isGlimtModerator("member-1")
	}
	if len(people.asked) != 3 {
		t.Errorf("projection asked %d times for 3 calls (%v) — a cached answer cannot be revoked",
			len(people.asked), people.asked)
	}
}

func TestIsGlimtModeratorFailsClosed(t *testing.T) {
	// No person row: an id we do not recognise must not moderate.
	notFound := &stubPeople{found: false, p: person.Person{SectionSlug: person.SectionTeam}}
	app := photoTestApp(t, &cqrstest.Publisher{}, notFound)
	if app.isGlimtModerator("ghost") {
		t.Error("an unknown person moderates")
	}

	// No projection at all. Running without a database is a supported mode (PRD 008 §5),
	// and the safe answer there is "cannot hide other people's photos".
	noDB := photoTestApp(t, &cqrstest.Publisher{}, nil)
	if noDB.isGlimtModerator("member-1") {
		t.Error("moderation granted with no person projection")
	}

	// Empty id — what an unauthenticated or half-built request looks like.
	empty := &stubPeople{found: true, p: person.Person{SectionSlug: person.SectionTeam}}
	appEmpty := photoTestApp(t, &cqrstest.Publisher{}, empty)
	if appEmpty.isGlimtModerator("") {
		t.Error("empty person id moderates")
	}

	// A member with no section assigned. Routine upstream data, not an error.
	unassigned := &stubPeople{found: true, p: person.Person{SectionSlug: ""}}
	appUnassigned := photoTestApp(t, &cqrstest.Publisher{}, unassigned)
	if appUnassigned.isGlimtModerator("member-1") {
		t.Error("a member with no section moderates")
	}
}

func TestGlimtViewerCarriesTheModeratorFlag(t *testing.T) {
	people := &stubPeople{found: true, p: person.Person{SectionSlug: person.SectionTeam}}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)

	v := app.glimtViewer("member-1", "crew")
	if v.PersonID != "member-1" || v.Role != "crew" {
		t.Errorf("viewer = %+v, want the id and role passed in", v)
	}
	if !v.IsModerator {
		t.Error("glimtViewer did not carry the moderator flag")
	}
}
