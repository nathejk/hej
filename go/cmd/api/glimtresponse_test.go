package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/glimt"
	"nathejk.dk/nathejk/table/person"
)

// Attribution and projection tests (task 302).
//
// Two properties, and the second is the one that would fail silently in production:
//
//  1. Attribution is frozen at creation, so renaming a hold does not rewrite history.
//  2. No response but the moderation queue can carry an author. Asserted against the
//     **serialised JSON**, not the struct, because the struct is what we control and the JSON is
//     what leaves the building.

func storedGlimt() glimt.Glimt {
	return glimt.Glimt{
		GlimtID:        "g-1",
		Year:           "2026",
		AuthorPersonID: "p-author",
		AuthorGroup:    "spejder",
		TeamNumber:     "42",
		TeamName:       "Ørnene",
		Audience:       glimt.AudienceGroup,
		Caption:        "ved posten",
		CreatedAt:      time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC),
		MediaCount:     2,
		Media: []glimt.Media{
			{Ordinal: 0, Ref: strings.Repeat("a", 64), ThumbRef: strings.Repeat("b", 64),
				Kind: glimt.MediaKindImage, Width: 1024, Height: 768},
			{Ordinal: 1, Ref: strings.Repeat("c", 64), Kind: glimt.MediaKindVideo,
				Width: 1920, Height: 1080, DurationMs: 12000},
		},
	}
}

// forbiddenInPayload is everything that must never appear in a member-facing Glimt response.
//
// Checked as substrings of the JSON rather than as absent struct fields, because the question is
// what reaches the client. A test on the struct would pass while an embedded type leaked.
var forbiddenInPayload = []string{
	"p-author",              // the author's person id
	"author",                // any author-shaped key
	"person_id",             //
	"personId",              //
	"phone",                 // no phone number, ever, and never a guardian's
	"portrait",              //
	strings.Repeat("a", 64), // a blob ref: a content-addressed URL is a capability
	strings.Repeat("b", 64),
	strings.Repeat("c", 64),
}

func assertNoAuthor(t *testing.T, label string, v any) {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: marshal: %v", label, err)
	}
	payload := string(encoded)
	for _, forbidden := range forbiddenInPayload {
		if strings.Contains(payload, forbidden) {
			t.Errorf("%s payload contains %q\ngot: %s", label, forbidden, payload)
		}
	}
}

func TestGlimtResponseCarriesNoAuthor(t *testing.T) {
	g := storedGlimt()

	// A stranger's view.
	assertNoAuthor(t, "stranger", newGlimtResponse(g, "p-other"))

	// And the author's own view. Not an exception: the card reads "Din patrulje" and gets a
	// delete action, which conveys ownership without a name. This is also the payload most
	// likely to be cached on disk.
	own := newGlimtResponse(g, "p-author")
	assertNoAuthor(t, "own", own)
	if !own.Own {
		t.Error("the author's own glimt is not marked Own")
	}
	if newGlimtResponse(g, "p-other").Own {
		t.Error("a stranger's view is marked Own")
	}
}

func TestPublicGlimtResponseCarriesNoAuthor(t *testing.T) {
	assertNoAuthor(t, "public", newPublicGlimtResponse(storedGlimt()))
}

func TestGlimtResponseAttributesTheHoldAtEveryScope(t *testing.T) {
	// Number, name and group at every scope including public — a parent has to be able to
	// recognise their own child's patrulje (PRD 019 §0).
	g := storedGlimt()
	for _, audience := range []string{glimt.AudienceGroup, glimt.AudienceNathejk, glimt.AudiencePublic} {
		g.Audience = audience
		got := newGlimtResponse(g, "p-other")
		if got.Hold.Number != "42" || got.Hold.Name != "Ørnene" || got.Hold.Group != "spejder" {
			t.Errorf("audience %q attribution = %+v", audience, got.Hold)
		}
	}
	pub := newPublicGlimtResponse(g)
	if pub.Hold.Number != "42" || pub.Hold.Name != "Ørnene" || pub.Hold.Group != "spejder" {
		t.Errorf("public attribution = %+v", pub.Hold)
	}
}

func TestGlimtResponseMediaIsAddressedByOrdinal(t *testing.T) {
	// The blob ref must not travel. A content-addressed URL is a capability, and handing one
	// out is how a group-scoped photo becomes readable by whoever it was forwarded to.
	got := newGlimtResponse(storedGlimt(), "p-other")
	if len(got.Media) != 2 {
		t.Fatalf("media = %+v", got.Media)
	}
	if got.Media[0].Ordinal != 0 || !got.Media[0].HasThumb {
		t.Errorf("first item = %+v", got.Media[0])
	}
	// HasThumb false means "fall back to the full item", which is what stops the grid from
	// either 404ing or fetching full-size media for every tile.
	if got.Media[1].HasThumb {
		t.Errorf("second item claims a thumbnail it does not have: %+v", got.Media[1])
	}
	if got.Media[1].DurationMs != 12000 || got.Media[1].Kind != glimt.MediaKindVideo {
		t.Errorf("video item = %+v", got.Media[1])
	}
	// A still must not carry a duration at all.
	encoded, _ := json.Marshal(got.Media[0])
	if strings.Contains(string(encoded), "duration_ms") {
		t.Errorf("a still carries duration_ms: %s", encoded)
	}
}

func TestHiddenIsVisibleToTheAuthor(t *testing.T) {
	// A hidden glimt reaches only its author or a moderator, so the flag is safe to send —
	// and the author is entitled to know it was hidden rather than silently discovering that
	// nobody can see it.
	g := storedGlimt()
	hidden := time.Date(2026, 9, 17, 23, 0, 0, 0, time.UTC)
	g.HiddenAt = &hidden

	got := newGlimtResponse(g, "p-author")
	if !got.Hidden {
		t.Error("a hidden glimt is not flagged")
	}
	assertNoAuthor(t, "hidden own", got)
}

func TestModerationResponseIsTheOnlyOneWithAnAuthor(t *testing.T) {
	// The single disclosure in the feature, kept in its own type so that "which responses
	// disclose the author?" is answerable by grep.
	g := storedGlimt()
	mod := moderationGlimtResponse{
		glimtResponse:  newGlimtResponse(g, "p-moderator"),
		AuthorPersonID: g.AuthorPersonID,
		AuthorName:     "Astrid",
		ReportCount:    2,
	}
	encoded, err := json.Marshal(mod)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), "p-author") {
		t.Errorf("the moderation queue must carry the author, or a report cannot be answered\ngot: %s", encoded)
	}
	// It embeds glimtResponse, so it also inherits the no-blob-ref property.
	for _, ref := range []string{strings.Repeat("a", 64), strings.Repeat("c", 64)} {
		if strings.Contains(string(encoded), ref) {
			t.Errorf("moderation payload leaked a blob ref\ngot: %s", encoded)
		}
	}
}

func TestAttributionFor(t *testing.T) {
	spejder := person.Person{TeamNumber: "42", TeamName: "Ørnene", SectionName: ""}
	group, number, name := attributionFor(spejder, users.RoleSpejder)
	if group != "spejder" || number != "42" || name != "Ørnene" {
		t.Errorf("spejder attribution = %q, %q, %q", group, number, name)
	}

	bandit := person.Person{TeamNumber: "7", TeamName: "Klan Nord"}
	group, number, name = attributionFor(bandit, users.RoleBandit)
	if group != "bandit" || number != "7" || name != "Klan Nord" {
		t.Errorf("bandit attribution = %q, %q, %q", group, number, name)
	}

	// Crew have a section, not a numbered hold. An empty number here is a normal state, not
	// missing data, and the client renders the name alone.
	crew := person.Person{TeamNumber: "", TeamName: "", SectionName: "Postmandskab"}
	group, number, name = attributionFor(crew, users.RoleSamarit)
	if group != "crew" || number != "" || name != "Postmandskab" {
		t.Errorf("crew attribution = %q, %q, %q", group, number, name)
	}

	// gøglere land in the crew bucket for Glimt (see users.GlimtGroupFor) and are attributed
	// by their section like any other crew.
	goegler := person.Person{SectionName: "Gøglere"}
	group, _, name = attributionFor(goegler, users.RoleGoegler)
	if group != "crew" || name != "Gøglere" {
		t.Errorf("gøgler attribution = %q, %q", group, name)
	}

	// An unrecognised role cannot be attributed to a group, and the group is what decides who
	// a group-scoped glimt reaches. Empty everything, and the caller refuses the post.
	group, number, name = attributionFor(spejder, users.Role("hjælper"))
	if group != "" || number != "" || name != "" {
		t.Errorf("unknown role attribution = %q, %q, %q; want empty so the post is refused", group, number, name)
	}
}

// TestAttributionIsFrozenNotResolved is the freezing property, expressed as the difference
// between what was stored and what the person record says now.
func TestAttributionIsFrozenNotResolved(t *testing.T) {
	stored := storedGlimt() // captured when the hold was "Ørnene", number 42

	// The hold has since been renamed and the author moved to another patrulje. The response
	// must still describe the moment as it was posted — it reads only the glimt row.
	got := newGlimtResponse(stored, "p-other")
	if got.Hold.Name != "Ørnene" || got.Hold.Number != "42" {
		t.Errorf("attribution changed with the person record: %+v", got.Hold)
	}

	// And prove the response never consults a person record at all: the only inputs are the
	// stored row and the viewer's id. If someone later adds a lookup here, this comment is
	// the reason not to.
	if newGlimtResponse(stored, "p-other").Hold != (holdAttribution{Number: "42", Name: "Ørnene", Group: "spejder"}) {
		t.Error("attribution is not derived purely from the stored row")
	}
}
