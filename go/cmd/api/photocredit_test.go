package main

import (
	"net/http"
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

// Crediting a crew member by reference (PRD 025 §6 R1, task 453).
func TestAdminCreditsACrewMemberByReference(t *testing.T) {
	_, srv, pub := positionApp(t)

	body := `{"photoIds":["` + photoID("a") + `"],"creditCrewId":"user-7"}`
	resp := patchAdmin(t, srv, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	if len(pub.Messages) != 1 {
		t.Fatalf("want one event, got %d", len(pub.Messages))
	}

	var out photo.Updated
	if err := pub.Messages[0].Body(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.CreditCrewID == nil || *out.CreditCrewID != "user-7" {
		t.Errorf("the event must carry the reference, got %+v", out.CreditCrewID)
	}
	// **And no name.** The log is append-only, so a resolved name written here could never be erased — which is
	// the one place the erasure argument for this whole feature has no answer.
	if out.Credit != nil {
		t.Errorf("the event must not carry a typed credit as well, got %q", *out.Credit)
	}
}

// The two forms are mutually exclusive on the way in, like every other action on this endpoint.
//
// A request carrying both would be a curator saying two things at once, and the endpoint answering "I picked one
// for you" is how a photograph ends up credited to somebody nobody chose.
func TestAdminRefusesBothCreditFormsAtOnce(t *testing.T) {
	_, srv, pub := positionApp(t)

	body := `{"photoIds":["` + photoID("a") + `"],"credit":"Foto: En Gæst","creditCrewId":"user-7"}`
	if resp := patchAdmin(t, srv, body); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
	if len(pub.Messages) != 0 {
		t.Errorf("a refused request must publish nothing, got %d", len(pub.Messages))
	}
}

// An id longer than the column is refused rather than truncated.
//
// The opposite of the typed credit, which is truncated: a shortened *name* is still recognisably a name, while a
// shortened id is a different id — one that resolves to nobody, or to somebody else.
func TestAdminRefusesAnOverlongCrewReference(t *testing.T) {
	_, srv, pub := positionApp(t)

	body := `{"photoIds":["` + photoID("a") + `"],"creditCrewId":"` + strings.Repeat("x", 100) + `"}`
	if resp := patchAdmin(t, srv, body); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
	if len(pub.Messages) != 0 {
		t.Errorf("a refused request must publish nothing, got %d", len(pub.Messages))
	}
}

// The sheet offers both routes, and says what is different about them (PRD 025 §7).
//
// Source-read, because there is no JavaScript runtime here. What is worth pinning is not the markup's shape but
// the two sentences a curator needs: that a picked name is spelled consistently and can be erased, and that a
// typed one cannot be corrected later. Those are the reason the picker exists, and copy is the only place they
// are said.
func TestTheCreditSheetOffersBothRoutesAndSaysWhatDiffers(t *testing.T) {
	page := adminPageSource(t)

	for _, want := range []struct{ needle, why string }{
		{`<select id="creditcrew"`, "the roster list"},
		{`<input type="search" id="crewsearch"`, "searchable, because a curator knows the name and not the position"},
		{`id="crewall"`, "and expandable: `pr` is the default, not the boundary"},
		{`<input type="text" id="credittext"`, "the typed path stays, for a guest photographer"},
		{"staves navnet altid ens", "the picker's first reason: consistent spelling"},
		{"forsvinder navnet fra alle billederne", "and its second: erasure in one place"},
		{"bliver <strong>ikke</strong>\n      rettet automatisk", "the typed path's consequence, said where the " +
			"choice is made rather than discovered later"},
		{"hvor alle kan læse det", "and that this is public, which was already here and must stay"},
	} {
		if !strings.Contains(page, want.needle) {
			t.Errorf("the credit sheet is missing %q: %s", want.needle, want.why)
		}
	}

	// Two buttons, so picking and typing are separate acts rather than one button guessing which was meant.
	for _, id := range []string{"docreditcrew", "docredit", "doclearcredit"} {
		if !strings.Contains(page, `id="`+id+`"`) {
			t.Errorf("the sheet must have a %q button", id)
		}
	}
}

// The tool sends the reference and never a resolved name.
func TestTheCreditSheetSendsTheReferenceNotTheName(t *testing.T) {
	js := stripJSLineComments(adminAsset(t, "creditaction.js"))

	if !strings.Contains(js, "sendCredit({ creditCrewId: id });") {
		t.Error("picking a photographer must send the id: sending the name would put it on the append-only log, " +
			"where it could not be erased")
	}
	// The roster's names are rendered into the list and nowhere else — in particular they are never put in the
	// request body, and never remembered on this machine.
	if strings.Contains(js, "CREDIT_KEY, id") || strings.Contains(js, "setItem(CREDIT_KEY, member") {
		t.Error("a picked credit must not be remembered in localStorage: the list is the default, and an id in " +
			"browser storage would be a person reference sitting there for no reason")
	}
	// The list is fetched when the sheet opens, not on page load: `/admin` is `no-store`, so a roster fetched on
	// load would be a request nobody asked for on every page view.
	if !strings.Contains(js, "loadCrew();") {
		t.Error("the roster must be fetched when the sheet opens")
	}
}
