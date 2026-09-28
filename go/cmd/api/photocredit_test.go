package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/person"
	"nathejk.dk/nathejk/table/photo"
)

// A credit by crew reference, on the public page and in the curator's tool (PRD 025 §6 R5/R6, task 452).
//
// # What is worth testing here, given that `creditnames_test.go` covers the bounds
//
// Two things, and neither is about SQL:
//
//  1. that a reference becomes a **name** before the page sees it — the page must not be able to tell a picked
//     credit from a typed one, which is the whole point of the feature;
//  2. that the **reference itself** never reaches the page. It is a handle to a person record, and a public page
//     holding one is a different thing from a public page holding a name somebody agreed to.

// creditPeople is a person projection that resolves exactly the ids it was given, and records what it was asked.
type creditPeople struct {
	person.Queries

	names map[string]string
	asked []string
	years []string
}

func (c *creditPeople) CreditNames(year string, ids []string) (map[string]string, error) {
	c.years = append(c.years, year)
	c.asked = append(c.asked, ids...)

	out := map[string]string{}
	for _, id := range ids {
		if name, ok := c.names[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

func TestAlbumPageRendersACreditedCrewMembersName(t *testing.T) {
	app, store := albumApp(t)
	people := &creditPeople{names: map[string]string{"user-7": "Anne Sørensen"}}
	app.models.People = people

	// The first album's second item has no credit at all; give it a reference instead of a typed line.
	store.albums[0].items[1].CreditCrewID = "user-7"

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	if !strings.Contains(page, `<span class="credit">Anne Sørensen</span>`) {
		t.Errorf("a credit by reference must render as the crew member's name, in the same element a typed "+
			"credit renders in — the page must not be able to tell the two apart\n%s", page)
	}
	// **The reference itself is nowhere on the page.** Not in an attribute, not in a comment, not in the
	// filmstrip's data. PRD 025 §6 R6: the id is a handle to a person record.
	if strings.Contains(page, "user-7") {
		t.Errorf("the crew reference reached the public page; only the resolved name may\n%s", page)
	}
	// Resolved for the photograph's year, which on the public site is the year it serves.
	if len(people.years) != 1 || people.years[0] != "2026" {
		t.Errorf("the credit must be resolved within the photograph's year, got %v", people.years)
	}
	// And only the window's references were asked for — not the whole album's.
	if len(people.asked) != 1 || people.asked[0] != "user-7" {
		t.Errorf("only the references on the page should be resolved, got %v", people.asked)
	}
}

// **The erasure path** (PRD 025 §6 R4/R5): a crew member who has been deleted resolves to nothing, and the page
// renders no credit line rather than an empty one.
//
// This is the behaviour that makes "one place to delete them, then they are gone" true, and it is why an
// unresolvable reference is an ordinary outcome rather than an error: somebody exercising that right must not
// produce a logged failure or a placeholder where their name was.
func TestAlbumPageRendersNoCreditWhenTheCrewMemberIsGone(t *testing.T) {
	app, store := albumApp(t)
	// The projection resolves nothing: the person row is deleted, or was never crew.
	app.models.People = &creditPeople{names: map[string]string{}}
	store.albums[0].items[1].CreditCrewID = "user-gone"

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	if strings.Contains(page, "user-gone") {
		t.Errorf("an unresolvable reference must not be rendered\n%s", page)
	}
	// An empty credit element would be a credit that says nothing — visible whitespace under a photograph,
	// implying somebody was named. The template's `{{if .Credit}}` is what prevents it; asserted so that a
	// change to this handler cannot start passing "" as a credit worth rendering.
	if strings.Contains(page, `<span class="credit"></span>`) {
		t.Errorf("a credit that resolved to nothing must render no element at all\n%s", page)
	}
	// The photograph itself is untouched: erasure removes a name, not somebody's work.
	if !strings.Contains(page, "/api/public/albums/al-1/media/1?variant=thumb") {
		t.Errorf("the photograph must still be on the page\n%s", page)
	}
}

// A typed credit still works, and is not disturbed by the resolver existing. The fixture's first item carries
// one; this is the regression guard for the path that already existed.
func TestAlbumPageStillRendersATypedCredit(t *testing.T) {
	app, _ := albumApp(t)
	app.models.People = &creditPeople{names: map[string]string{"user-7": "Somebody Else"}}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	if page := string(body); !strings.Contains(page, fixtureCredit) {
		t.Errorf("a typed credit must be unaffected by the reference path\n%s", page)
	}
}

// The page must not fail because a credit could not be resolved.
//
// An unavailable person projection is the same outcome as a deleted crew member — no name — and it lands on the
// same path. The alternative would be a public album offline because a *credit line* could not be read.
func TestAlbumPageSurvivesAnUnavailablePersonProjection(t *testing.T) {
	app, store := albumApp(t)
	app.models.People = nil
	store.albums[0].items[1].CreditCrewID = "user-7"

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("the album page must still render, got %d", resp.StatusCode)
	}
	if page := string(body); !strings.Contains(page, "/api/public/albums/al-1/media/1?variant=thumb") {
		t.Errorf("the photographs must still be there\n%s", page)
	}
}

// The curator's tool shows the **resolved** name too, so it shows what the public will show.
//
// Not the reference, which is meaningless to read, and not the typed field, which is empty on a photograph
// credited by picker. It is also how a curator finds out that a credit has stopped resolving because the crew
// member asked to be deleted.
func TestTheLibraryReadShowsTheResolvedCredit(t *testing.T) {
	curator := &libraryCurator{rows: []photo.LibraryPhoto{
		{ID: photoID("a"), CreditCrewID: "user-7"},
		{ID: photoID("b"), Credit: "Foto: En Gæst"},
	}}
	app, srv := libraryApp(t, curator)
	app.models.People = &creditPeople{names: map[string]string{"user-7": "Anne Sørensen"}}

	out := decodeLibrary(t, getAdmin(t, srv, "/api/admin/photos", testAdminUser, testAdminPass))
	if len(out.Photos) != 2 {
		t.Fatalf("want two photographs, got %d", len(out.Photos))
	}

	byID := map[string]string{}
	for _, p := range out.Photos {
		byID[p.ID] = p.Credit
	}
	if byID[photoID("a")] != "Anne Sørensen" {
		t.Errorf("the tool must show the resolved name, got %q", byID[photoID("a")])
	}
	if byID[photoID("b")] != "Foto: En Gæst" {
		t.Errorf("a typed credit must be shown as typed, got %q", byID[photoID("b")])
	}

	// And the response carries no reference. `isPersonShaped` flags the field name, so a response type growing
	// one fails the privacy walk — this is the same rule asserted on the wire.
	body := adminBody(t, getAdmin(t, srv, "/api/admin/photos", testAdminUser, testAdminPass))
	if strings.Contains(body, "user-7") || strings.Contains(body, "creditCrewId") {
		t.Errorf("the library response must carry the resolved name and no reference\n%s", body)
	}
}
