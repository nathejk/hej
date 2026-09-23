package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/nathejk/table/album"
)

// The curator's album writes (PRD 022 §6, task 375).

// albumCurator is a stub album read model holding albums and their memberships.
type albumCurator struct {
	albums map[string]*curatedAlbum
	err    error

	// nextOrdinalCalls records the albums NextOrdinal was asked about, so a test can assert the allocation
	// actually consulted the album rather than assuming a base.
	nextOrdinalCalls []string
}

type curatedAlbum struct {
	a     album.CuratorAlbum
	items []album.CuratorItem
}

func newAlbumCurator(albums ...*curatedAlbum) *albumCurator {
	c := &albumCurator{albums: map[string]*curatedAlbum{}}
	for _, a := range albums {
		c.albums[a.a.ID] = a
	}
	return c
}

func (c *albumCurator) All(string) ([]album.CuratorAlbum, error) {
	if c.err != nil {
		return nil, c.err
	}
	out := []album.CuratorAlbum{}
	for _, a := range c.albums {
		out = append(out, a.a)
	}
	return out, nil
}

func (c *albumCurator) Album(_, albumID string) (album.CuratorAlbum, []album.CuratorItem, bool, error) {
	if c.err != nil {
		return album.CuratorAlbum{}, nil, false, c.err
	}
	a, ok := c.albums[albumID]
	if !ok {
		return album.CuratorAlbum{}, nil, false, nil
	}
	return a.a, a.items, true, nil
}

func (c *albumCurator) SlugTaken(_, slug string) (bool, error) {
	if c.err != nil {
		return false, c.err
	}
	for _, a := range c.albums {
		if a.a.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

func (c *albumCurator) NextOrdinal(_, albumID string) (int, error) {
	c.nextOrdinalCalls = append(c.nextOrdinalCalls, albumID)
	if c.err != nil {
		return 0, c.err
	}
	a, ok := c.albums[albumID]
	if !ok {
		return 0, nil
	}
	// max+1 over **every** row including removed ones, matching the real querier: reusing a removed position
	// would resurrect that row's soft delete through the upsert.
	max := -1
	for _, it := range a.items {
		if it.Ordinal > max {
			max = it.Ordinal
		}
	}
	return max + 1, nil
}

// albumWriteApp returns an admin app with the given album read model and a recording publisher.
func albumWriteApp(t *testing.T, curator *albumCurator) (*application, *httptest.Server, *cqrstest.Publisher) {
	t.Helper()

	app, srv := adminApp(t)
	app.models.AlbumCurator = curator
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	return app, srv, pub
}

// postAdmin sends a JSON body to an admin endpoint with the credential.
func postAdmin(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth(testAdminUser, testAdminPass)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func photoID(c string) string { return strings.Repeat(c, 64) }

// ---------------------------------------------------------------------------
// Creating an album.
// ---------------------------------------------------------------------------

func TestAdminCreateAlbum(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator())

	resp := postAdmin(t, srv, "/api/admin/albums", `{"title":"Lørdag morgen","sortOrder":10}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}

	var out createAdminAlbumResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.Slug != "loerdag-morgen" {
		t.Errorf("want the Danish letters folded conventionally, got %q", out.Slug)
	}
	if out.AlbumID == "" {
		t.Error("want an album id")
	}
	if out.AlbumID == out.Slug {
		t.Error("the id must not be the slug: the slug is the address and is frozen, the id is the identity")
	}

	if got := len(pub.Subjects()); got != 1 {
		t.Fatalf("want 1 event, got %d", got)
	}
	if !strings.Contains(pub.Subjects()[0], ".album."+out.AlbumID+".created") {
		t.Errorf("unexpected subject %q", pub.Subjects()[0])
	}
}

// **An album is always created unpublished.** Not a default the caller may override: an album is assembled over
// several sittings, and a create that could publish would put the first photograph on the open web before the
// second was chosen.
//
// Three layers hold this, and the test checks all three rather than just the outcome:
//
//  1. The request type has no `published` field, so there is nothing to set.
//  2. `ReadJSON` sets `DisallowUnknownFields`, so a client that tries anyway gets a **400** rather than being
//     silently ignored — which is better than tolerating it, because a client author who believes they can
//     publish on create should be told they cannot.
//  3. `album.Created` carries no published field either (task 363), so publishing is inexpressible as a create.
func TestAdminCreateAlbumIsAlwaysUnpublished(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator())

	resp := postAdmin(t, srv, "/api/admin/albums", `{"title":"En titel"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	var out createAdminAlbumResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.Published {
		t.Error("an album must be created unpublished")
	}

	// Asking for it is refused rather than ignored.
	for _, body := range []string{
		`{"title":"En anden","published":true}`,
		`{"title":"En tredje","Published":true}`,
	} {
		if got := postAdmin(t, srv, "/api/admin/albums", body).StatusCode; got != http.StatusBadRequest {
			t.Errorf("%s: a client asking to publish on create should get 400, got %d", body, got)
		}
	}

	// And the request type has no field for it, which is what makes the above true by construction rather than
	// by the handler remembering.
	for _, f := range structFieldNames(createAdminAlbumRequest{}) {
		if strings.EqualFold(f, "published") {
			t.Error("createAdminAlbumRequest must have no published field; publishing is a separate edit")
		}
	}

	// The event shape is the third layer.
	for _, f := range structFieldNames(album.Created{}) {
		if strings.EqualFold(f, "published") {
			t.Error("album.Created must have no published field; a replayed create must not republish an album")
		}
	}
}

func TestAdminCreateAlbumRefusesABadTitle(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator())

	for name, body := range map[string]string{
		"empty":            `{"title":""}`,
		"only spaces":      `{"title":"   "}`,
		"only punctuation": `{"title":"!!! ???"}`,
		"only emoji":       `{"title":"🎉🎉"}`,
		"too long":         `{"title":"` + strings.Repeat("a", maxAdminAlbumTitle+1) + `"}`,
	} {
		resp := postAdmin(t, srv, "/api/admin/albums", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, resp.StatusCode)
		}
	}
	if got := len(pub.Subjects()); got != 0 {
		t.Errorf("a refused create must publish nothing, got %d events", got)
	}
}

// A taken slug is refused with a reason rather than surfacing a database error on the insert. Deleted albums
// count, because the slug is unique per year and an undeleted one would collide.
func TestAdminCreateAlbumRefusesATakenSlug(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "natten", Title: "Natten", Deleted: true,
	}})
	_, srv, _ := albumWriteApp(t, curator)

	resp := postAdmin(t, srv, "/api/admin/albums", `{"title":"Natten"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 for a taken slug, got %d", resp.StatusCode)
	}
	if body := adminBody(t, resp); !strings.Contains(body, "natten") {
		t.Errorf("the reason should name the slug, got %q", body)
	}
}

func TestSlugifyAlbumTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Lørdag morgen":        "loerdag-morgen",
		"Ved Målet":            "ved-maalet",
		"Æblerne":              "aeblerne",
		"Natten":               "natten",
		"Post 3":               "post-3",
		"  mange   mellemrum ": "mange-mellemrum",
		"Tegn!?&og/skråstreg":  "tegn-og-skraastreg",
		"-foran og bagved-":    "foran-og-bagved",
		"!!!":                  "",
	} {
		if got := slugifyAlbumTitle(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// The Danish letters get their conventional two-letter forms rather than being stripped or ASCII-folded. A
// generic Unicode decomposition would give "lordag", which is a different word; stripping would give "lrdag",
// which is not a slug anybody would have typed.
func TestSlugifyFoldsDanishLettersConventionally(t *testing.T) {
	got := slugifyAlbumTitle("Lørdag på åen med æg")
	if strings.Contains(got, "lordag") {
		t.Errorf("ø must fold to oe, not o: %q", got)
	}
	if got != "loerdag-paa-aaen-med-aeg" {
		t.Errorf("got %q", got)
	}
}

// Every slug this produces must be one the projection will accept, or the album is created with an address
// nobody can reach and nothing says why. The fold refuses an unusable slug outright (task 363).
func TestEverySlugWeProduceIsOneTheFoldAccepts(t *testing.T) {
	for _, title := range []string{
		"Lørdag morgen", "Ved Målet", "Post 3", "Æ Ø Å", "a", "Natten 2026",
		"Meget lang titel med mange ord i den og flere endnu",
		"UPPERCASE", "tegn!?&/()", "123", "dobbelt--bindestreg",
	} {
		slug := slugifyAlbumTitle(title)
		if slug == "" {
			continue // refused before publishing, which is the other acceptable outcome
		}
		// The same rules album.validSlug applies: lowercase letters, digits, hyphens, no leading or trailing
		// hyphen, no doubled hyphen, at most 64 characters.
		if len(slug) > 64 {
			t.Errorf("%q gave an over-long slug %q", title, slug)
		}
		if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") || strings.Contains(slug, "--") {
			t.Errorf("%q gave a slug the fold would refuse: %q", title, slug)
		}
		for _, r := range slug {
			if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
				t.Errorf("%q gave a slug with an illegal character: %q", title, slug)
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Adding a selection to albums.
// ---------------------------------------------------------------------------

func TestAdminAddsASelectionToOneAlbum(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Title: "Natten"}})
	_, srv, pub := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("a") + `","` + photoID("c") + `"],"albumIds":["al-1"]}`
	resp := postAdmin(t, srv, "/api/admin/albums/items", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}

	var out addAdminAlbumItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(out.Albums) != 1 || out.Albums[0].Added != 2 || out.Albums[0].AlreadyThere != 0 {
		t.Errorf("want 2 added to one album, got %+v", out.Albums)
	}
	if got := len(pub.Subjects()); got != 2 {
		t.Errorf("want one event per pair, got %d", got)
	}
}

// **The action the whole model change exists for.** One photograph into three albums is three membership rows
// referencing one photograph — not three copies of it (PRD 022 §2, §8.3).
func TestAdminAddsOnePhotographToSeveralAlbums(t *testing.T) {
	curator := newAlbumCurator(
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Title: "Natten"}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-2", Title: "Målet"}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-3", Title: "Postmandskabet"}},
	)
	_, srv, pub := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("a") + `"],"albumIds":["al-1","al-2","al-3"]}`
	resp := postAdmin(t, srv, "/api/admin/albums/items", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	if got := len(pub.Subjects()); got != 3 {
		t.Fatalf("want one event per album, got %d", got)
	}
	// Every event names the same photograph. That is the property: one photograph, several memberships.
	for _, msg := range pub.Messages {
		var body album.ItemAdded
		if err := msg.Body(&body); err != nil {
			t.Fatalf("decoding the event: %v", err)
		}
		if body.PhotoID != photoID("a") {
			t.Errorf("want the same photoId on every membership, got %q", body.PhotoID)
		}
	}
}

// **Re-adding an existing member is a no-op**: no duplicate row, no new event, and no ordinal churn.
// Re-selecting is routine when a curator works through a filter over several sittings.
func TestAdminReAddingAMemberIsANoOp(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Title: "Natten"},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a")},
			{Ordinal: 1, PhotoID: photoID("c")},
		},
	})
	_, srv, pub := albumWriteApp(t, curator)

	// Two already there, one new.
	body := `{"photoIds":["` + photoID("a") + `","` + photoID("c") + `","` + photoID("d") + `"],` +
		`"albumIds":["al-1"]}`
	resp := postAdmin(t, srv, "/api/admin/albums/items", body)

	var out addAdminAlbumItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.Albums[0].Added != 1 || out.Albums[0].AlreadyThere != 2 {
		t.Errorf("want 1 added and 2 already there, got %+v", out.Albums[0])
	}
	if got := len(pub.Subjects()); got != 1 {
		t.Errorf("only the new photograph should publish, got %d events", got)
	}

	// And the new one appends after the maximum rather than reusing 0 or 1.
	var added album.ItemAdded
	if err := pub.Messages[0].Body(&added); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if added.Ordinal != 2 {
		t.Errorf("want ordinal 2 appended after the existing maximum, got %d", added.Ordinal)
	}
}

// A membership the curator **removed** is not "already there" — it needs an event to clear the removal — but it
// must not take a second ordinal either, or the album would hold the photograph twice once the fold runs.
// Re-publishing at its existing ordinal clears `deleted` and puts it back where it was.
func TestAdminReinstatesARemovedMembershipInPlace(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Title: "Natten"},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a")},
			{Ordinal: 1, PhotoID: photoID("c"), Removed: true},
		},
	})
	_, srv, pub := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("c") + `"],"albumIds":["al-1"]}`
	postAdmin(t, srv, "/api/admin/albums/items", body)

	if got := len(pub.Subjects()); got != 1 {
		t.Fatalf("want 1 event to reinstate the membership, got %d", got)
	}
	var added album.ItemAdded
	if err := pub.Messages[0].Body(&added); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if added.Ordinal != 1 {
		t.Errorf("a reinstated membership keeps its position: want ordinal 1, got %d", added.Ordinal)
	}
}

// Ordinals are appended after the maximum **including removed positions**, so a removed item's position is never
// reused. Reusing it would resurrect that row's soft delete through the upsert — silently putting a taken-down
// photograph back on the page.
func TestAdminNeverReusesARemovedOrdinal(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Title: "Natten"},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a")},
			{Ordinal: 1, PhotoID: photoID("c"), Removed: true},
			{Ordinal: 2, PhotoID: photoID("d"), Removed: true},
		},
	})
	_, srv, pub := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("e") + `"],"albumIds":["al-1"]}`
	postAdmin(t, srv, "/api/admin/albums/items", body)

	var added album.ItemAdded
	if err := pub.Messages[0].Body(&added); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if added.Ordinal != 3 {
		t.Errorf("want ordinal 3, after the removed positions; got %d — reusing 1 or 2 would undelete them",
			added.Ordinal)
	}
}

// Ordinals are allocated per album, and the allocation consults each album rather than sharing a base.
func TestAdminAllocatesOrdinalsPerAlbum(t *testing.T) {
	curator := newAlbumCurator(
		&curatedAlbum{
			a:     album.CuratorAlbum{ID: "al-1"},
			items: []album.CuratorItem{{Ordinal: 0, PhotoID: photoID("a")}, {Ordinal: 1, PhotoID: photoID("c")}},
		},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-2"}},
	)
	_, srv, pub := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("d") + `"],"albumIds":["al-1","al-2"]}`
	postAdmin(t, srv, "/api/admin/albums/items", body)

	got := map[string]int{}
	for _, msg := range pub.Messages {
		var added album.ItemAdded
		if err := msg.Body(&added); err != nil {
			t.Fatalf("decoding: %v", err)
		}
		got[added.AlbumID] = added.Ordinal
	}
	if got["al-1"] != 2 {
		t.Errorf("al-1 already holds 0 and 1, so want ordinal 2, got %d", got["al-1"])
	}
	if got["al-2"] != 0 {
		t.Errorf("al-2 is empty, so want ordinal 0, got %d", got["al-2"])
	}
	if len(curator.nextOrdinalCalls) != 2 {
		t.Errorf("the allocation must consult each album, got %v", curator.nextOrdinalCalls)
	}
}

// A batch into one album gets consecutive ordinals, gap-free, in the order the selection was given.
func TestAdminAppendsABatchConsecutively(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a:     album.CuratorAlbum{ID: "al-1"},
		items: []album.CuratorItem{{Ordinal: 4, PhotoID: photoID("a")}},
	})
	_, srv, pub := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("c") + `","` + photoID("d") + `","` + photoID("e") + `"],` +
		`"albumIds":["al-1"]}`
	postAdmin(t, srv, "/api/admin/albums/items", body)

	var ordinals []int
	for _, msg := range pub.Messages {
		var added album.ItemAdded
		if err := msg.Body(&added); err != nil {
			t.Fatalf("decoding: %v", err)
		}
		ordinals = append(ordinals, added.Ordinal)
	}
	want := []int{5, 6, 7}
	if len(ordinals) != 3 {
		t.Fatalf("want 3 events, got %d", len(ordinals))
	}
	for i := range want {
		if ordinals[i] != want[i] {
			t.Errorf("ordinals = %v, want %v", ordinals, want)
			break
		}
	}
}

// A duplicated id in the selection produces one event, not two. The client sends a Set but JSON has none, and a
// duplicate would make the response's counts wrong — which is what the curator reads.
func TestAdminDedupesTheSelection(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1"}})
	_, srv, pub := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("a") + `","` + photoID("a") + `"],"albumIds":["al-1","al-1"]}`
	resp := postAdmin(t, srv, "/api/admin/albums/items", body)

	var out addAdminAlbumItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(out.Albums) != 1 || out.Albums[0].Added != 1 {
		t.Errorf("want one album and one addition, got %+v", out.Albums)
	}
	if got := len(pub.Subjects()); got != 1 {
		t.Errorf("want 1 event, got %d", got)
	}
}

func TestAdminAddRefusesAnEmptyOrOversizeSelection(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1"}})
	_, srv, pub := albumWriteApp(t, curator)

	for name, body := range map[string]string{
		"no photographs": `{"photoIds":[],"albumIds":["al-1"]}`,
		"no albums":      `{"photoIds":["` + photoID("a") + `"],"albumIds":[]}`,
		"blank ids":      `{"photoIds":["  "],"albumIds":["al-1"]}`,
	} {
		resp := postAdmin(t, srv, "/api/admin/albums/items", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, resp.StatusCode)
		}
	}
	if got := len(pub.Subjects()); got != 0 {
		t.Errorf("a refused request must publish nothing, got %d", got)
	}
}

// The pair count is bounded, because one request becomes one event per (album, photo) pair and the log is never
// rewritten. Exercised rather than only asserted structurally: a selection over the ceiling is refused, and
// nothing is published.
func TestAdminAddRefusesTooManyPairs(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1"}})
	_, srv, pub := albumWriteApp(t, curator)

	// Distinct ids, one over the ceiling, into a single album.
	ids := make([]string, 0, maxAdminSelection+1)
	for i := 0; i <= maxAdminSelection; i++ {
		ids = append(ids, `"`+fmt.Sprintf("%064x", i)+`"`)
	}
	body := `{"photoIds":[` + strings.Join(ids, ",") + `],"albumIds":["al-1"]}`

	resp := postAdmin(t, srv, "/api/admin/albums/items", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 for a selection over the ceiling, got %d", resp.StatusCode)
	}
	if got := len(pub.Subjects()); got != 0 {
		t.Errorf("an over-large request must publish nothing, got %d", got)
	}
}

// The pair count is what becomes events, so it is what is bounded: twenty albums by two hundred photographs is
// four thousand messages on a log that is never rewritten.
func TestAdminAddBoundsThePairCount(t *testing.T) {
	src := adminSource(t, "adminalbum.go")
	if !strings.Contains(src, "len(photoIDs)*len(albumIDs) > maxAdminSelection") {
		t.Error("the bound must be on the pair count, not on either list alone: one request becomes one event " +
			"per (album, photo) pair")
	}
}

func TestAdminAddRefusesAnUnknownAlbum(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1"}})
	_, srv, pub := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("a") + `"],"albumIds":["al-nope"]}`
	resp := postAdmin(t, srv, "/api/admin/albums/items", body)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 for an unknown album, got %d", resp.StatusCode)
	}
	if got := len(pub.Subjects()); got != 0 {
		t.Errorf("an unknown album must publish nothing, got %d", got)
	}
}

// A broken stream must not report the selection as filed. The projection is downstream of the log, so a silent
// failure would look like success until the curator reloaded and found the album short.
func TestAdminAddWithNoStreamDoesNotClaimToHaveFiled(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1"}})
	app, srv, _ := albumWriteApp(t, curator)
	app.commands = commandsWithNoPublisher()

	body := `{"photoIds":["` + photoID("a") + `"],"albumIds":["al-1"]}`
	resp := postAdmin(t, srv, "/api/admin/albums/items", body)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("want 503 with no event stream, got %d", resp.StatusCode)
	}
}

func TestAdminAlbumWritesRequireTheCredential(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1"}})
	_, srv, _ := albumWriteApp(t, curator)

	for _, path := range []string{"/api/admin/albums", "/api/admin/albums/items"} {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-Proto", "https")
		// No credential.
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: want 401 without the credential, got %d", path, resp.StatusCode)
		}
	}
}

// Every write logs what happened, to what, and from where. With a shared credential the log is the only audit
// trail there is (PRD 022 §8.2).
func TestAdminAlbumWritesAreLogged(t *testing.T) {
	src := adminSource(t, "adminalbum.go")

	for _, want := range []string{
		`"admin created an album"`,
		`"admin added photographs to an album"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing the audit log line %s", want)
		}
	}
	if strings.Count(src, `"ip", clientIP(r)`) < 2 {
		t.Error("every album write must log the client IP")
	}
}
