package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// The album list, as an htmx fragment (task 395).
//
// # What changed, and why these tests are the point of the change
//
// The list used to be ~190 lines of JavaScript that fetched `/api/admin/albums` and assembled DOM nodes. Its
// rules were real — drafts and deleted albums shown rather than filtered (task 366), covers through the **admin**
// media route because the public one correctly refuses an unpublished album's photographs (task 382), publication
// posted alone (task 378) — and the only way to check any of them was to grep the script's own source text.
//
// That is a weak guard, and this repo has the scars to prove it: a needle matching the Go comment that explained
// the rule instead of the code implementing it, three times in one session. Worse, a source-text assertion cannot
// tell you the list *renders*; only that a string appears near some code.
//
// Rendered server-side, every one of those rules is observable in an HTTP response. These tests ask the endpoint
// for the list and read what a browser would get. That is the whole reason for adopting htmx here — not the line
// count.

// albumListFragment asks for the list and returns its HTML.
func albumListFragment(t *testing.T, srv *httptest.Server) string {
	t.Helper()

	resp := getAdmin(t, srv, "/admin/fragments/albums", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 from the album list fragment, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("a fragment is HTML for htmx to swap in, got Content-Type %q", ct)
	}
	return adminBody(t, resp)
}

// postAdminForm sends a form-encoded body to an admin endpoint with the credential.
//
// Form-encoded rather than JSON because that is what htmx posts without an extension, and adding an extension to
// send JSON would be a fourth vendored dependency bought to avoid a `r.FormValue`.
func postAdminForm(t *testing.T, srv *httptest.Server, path string, form url.Values) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth(testAdminUser, testAdminPass)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// **Each album links to its editor.** This was task 391's whole point and is worth keeping in the first test:
// `/admin/album/{slug}` — where publish, the title, the order and the captions live — had no link from anywhere
// and was reachable only by typing a slug into the address bar. The publish half of PRD 022 §5 was built and
// unusable. A feature with no route to it is indistinguishable from a missing one.
func TestTheAlbumListFragmentLinksEachAlbumToItsEditor(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "natten", Title: "Natten", Published: true, ItemCount: 4,
	}}))

	body := albumListFragment(t, srv)

	if !strings.Contains(body, `href="/admin/album/natten"`) {
		t.Errorf("each album must link to its editor\n%s", body)
	}
	// A published album also links to its public page, in the configured year — which is why the fragment needs
	// the year at all.
	if !strings.Contains(body, `href="/2026/album/natten"`) {
		t.Errorf("a published album should link to the page a family would see\n%s", body)
	}
	if !strings.Contains(body, "Natten") {
		t.Error("the album's title must appear")
	}
}

// The list is the **curator's** read: drafts and deleted albums are shown, not filtered.
//
// That is the entire difference between `album.CuratorQueries` and `album.Queries` (task 366). The public read
// hides them so drafts cannot be enumerated; this one shows them because "what have I not published yet" is the
// question a curator opens the page with.
func TestTheAlbumListFragmentShowsDraftsAndDeletedAlbums(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Slug: "udgivet", Title: "Udgivet album", Published: true, ItemCount: 4}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-2", Slug: "kladde", Title: "Kladde album", ItemCount: 1}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-3", Slug: "slettet", Title: "Slettet album", Deleted: true}},
	))

	body := albumListFragment(t, srv)

	for _, want := range []struct{ needle, why string }{
		{"Kladde album", "a draft must be listed at all"},
		{"Slettet album", "a deleted album must be listed: hiding it is what the public read does"},
		{">Kladde<", "a draft must say so"},
		{">Udgivet<", "a published album must say so"},
		{">Slettet<", "a deleted album must say so rather than looking like a draft"},
		{"1 billede<", "the count needs a singular; see task 387"},
		{"4 billeder", "the plural form for anything else"},
	} {
		if !strings.Contains(body, want.needle) {
			t.Errorf("the list is missing %q: %s\n%s", want.needle, want.why, body)
		}
	}

	// The note answers the question the page is opened with — not "3 album" but how many are unpublished.
	if !strings.Contains(body, "1 album er ikke udgivet endnu.") {
		t.Errorf("the note should count the drafts\n%s", body)
	}

	// A deleted album offers no publish button: restoring one is not built, and a button that would publish
	// something taken down is the wrong thing to offer.
	if strings.Contains(body, "/admin/fragments/albums/al-3/published") {
		t.Error("a deleted album must not get a publish button")
	}
	if !strings.Contains(body, "/admin/fragments/albums/al-2/published") {
		t.Error("a draft must get a publish button")
	}
}

// The cover comes through the **admin** media route, not the public one.
//
// The list shows unpublished albums, and the public media route would — correctly — refuse their photographs
// (task 382). A list whose draft covers were all broken images is a list a curator stops trusting.
func TestTheAlbumListFragmentCoversUseTheAdminMediaRoute(t *testing.T) {
	cover := photoID("a")
	_, srv, _ := albumWriteApp(t, newAlbumCurator(
		&curatedAlbum{a: album.CuratorAlbum{
			ID: "al-1", Slug: "kladde", Title: "Kladde", ItemCount: 2, CoverPhotoID: cover,
		}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-2", Slug: "tom", Title: "Tomt album"}},
	))

	body := albumListFragment(t, srv)

	if !strings.Contains(body, `src="/api/admin/photos/`+cover+`/media?variant=thumb"`) {
		t.Errorf("covers must be fetched through the admin media route\n%s", body)
	}
	if strings.Contains(body, "/api/public/albums/") {
		t.Error("the curator's list must not address the public media route")
	}
	// An album with no live items has no cover, and gets a placeholder rather than a blank gap — so the row
	// reads as "empty album" instead of "image failed to load".
	if !strings.Contains(body, `class="cover none"`) {
		t.Errorf("an album with no cover needs a placeholder, not an empty space\n%s", body)
	}
}

// Publishing works from the list, and publication is published **alone**.
//
// Task 378's reasoning, which this must not undo: a curator pressing publish has said one thing, so bundling it
// with the album's other fields would let a half-typed title ride along with the publication. Now checkable
// against the event rather than against the shape of a `JSON.stringify` call in a script.
func TestPublishingFromTheAlbumListFragmentPublishesPublicationAlone(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "kladde", Title: "Kladde", ItemCount: 2,
	}}))

	resp := postAdminForm(t, srv, "/admin/fragments/albums/al-1/published",
		url.Values{"published": {"true"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	body := adminBody(t, resp)

	if got := len(pub.Messages); got != 1 {
		t.Fatalf("want exactly 1 event, got %d", got)
	}
	if !strings.Contains(pub.Subjects()[0], ".album.al-1.updated") {
		t.Errorf("unexpected subject %q", pub.Subjects()[0])
	}

	// Decoded into a map rather than `album.Updated`, because the question is which keys are *present*: an
	// `album.Updated` would happily show nil for a field the event never carried, which is exactly the
	// distinction being tested.
	var payload map[string]json.RawMessage
	if err := pub.Messages[0].Body(&payload); err != nil {
		t.Fatalf("decoding the event: %v", err)
	}
	for _, forbidden := range []string{"title", "description", "sortOrder"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("publication must travel alone; the event also carried %q (task 378)", forbidden)
		}
	}
	if string(payload["published"]) != "true" {
		t.Errorf("want published:true, got %s", payload["published"])
	}

	// The fold is asynchronous and the frontpage is cached for a minute (task 335). Without saying so, a curator
	// presses the button again.
	if !strings.Contains(body, "inden for et minut") {
		t.Errorf("the delay before the frontpage updates must be stated\n%s", body)
	}
	// And the answer is the list itself, so the swap leaves the curator where they were.
	if !strings.Contains(body, `id="albums"`) {
		t.Errorf("the response must be the whole list: the swap is outerHTML\n%s", body)
	}
}

// Unpublishing is the same route with the other word, and nothing else.
func TestUnpublishingFromTheAlbumListFragmentPublishesPublicationAlone(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "udgivet", Title: "Udgivet", Published: true, ItemCount: 2,
	}}))

	resp := postAdminForm(t, srv, "/admin/fragments/albums/al-1/published",
		url.Values{"published": {"false"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}

	var payload map[string]json.RawMessage
	if err := pub.Messages[0].Body(&payload); err != nil {
		t.Fatalf("decoding the event: %v", err)
	}
	if string(payload["published"]) != "false" {
		t.Errorf("want published:false, got %s", payload["published"])
	}
}

// A value that is neither word is refused rather than defaulted.
//
// Defaulting would mean a mis-sent value quietly **unpublishing** an album, which is the one direction of this
// toggle that has a visible consequence for a family looking at the frontpage.
func TestTheAlbumListFragmentRefusesAnUnrecognisedPublishedValue(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "udgivet", Title: "Udgivet", Published: true,
	}}))

	for _, bad := range []url.Values{{}, {"published": {"1"}}, {"published": {"TRUE"}}} {
		resp := postAdminForm(t, srv, "/admin/fragments/albums/al-1/published", bad)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("want 400 for %v, got %d", bad, resp.StatusCode)
		}
	}
	if got := len(pub.Messages); got != 0 {
		t.Errorf("a refused request must publish nothing, got %d events", got)
	}
}

// Creating an album from the list, as a real form rather than the `window.prompt` it replaced.
//
// It publishes through the same helper the JSON endpoint uses, which is the rule for every fragment that writes:
// a fragment may *render* differently and must never *decide* differently. An event log that disagreed with
// itself depending on which button produced the write would be the worst possible outcome of adding htmx.
func TestCreatingAnAlbumFromTheListFragmentCreatesADraftAndReturnsTheList(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator())

	resp := postAdminForm(t, srv, "/admin/fragments/albums", url.Values{"title": {"Lørdag morgen"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	body := adminBody(t, resp)

	if got := len(pub.Messages); got != 1 {
		t.Fatalf("want 1 event, got %d", got)
	}
	subject := pub.Subjects()[0]
	if !strings.Contains(subject, ".album.") || !strings.HasSuffix(subject, ".created") {
		t.Errorf("unexpected subject %q", subject)
	}

	var created album.Created
	if err := pub.Messages[0].Body(&created); err != nil {
		t.Fatalf("decoding the event: %v", err)
	}
	// The same slug the JSON endpoint would have derived — the Danish letters folded conventionally.
	if created.Slug != "loerdag-morgen" {
		t.Errorf("want the shared slugify, got %q", created.Slug)
	}
	if created.Year != "2026" {
		t.Errorf("an album belongs to a year, got %q", created.Year)
	}

	// **A draft, said out loud.** Creating something that is not yet public, in a tool whose other button
	// publishes, is worth one sentence.
	if !strings.Contains(body, "oprettet som kladde") {
		t.Errorf("the outcome must say the album is a draft\n%s", body)
	}
	if !strings.Contains(body, `id="albums"`) {
		t.Errorf("the answer to a create is the list\n%s", body)
	}
}

// A refusal is shown **in the list**, not as an error page.
//
// The curator is looking at a list and mistyped a title, so the answer belongs where they are looking. A 4xx body
// htmx swapped in would replace the list with a bare sentence and lose it.
func TestTheAlbumListFragmentShowsACreateRefusalInTheList(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 1,
	}}))

	for _, tc := range []struct{ title, want string }{
		{"", "skal have en titel"},
		{"   ", "skal have en titel"},
		{strings.Repeat("a", maxAdminAlbumTitle+1), "for lang"},
		{"!!!", "kan ikke bruges i en adresse"},
		{"Natten", "“natten”"}, // the slug is taken, and the reason names it
	} {
		resp := postAdminForm(t, srv, "/admin/fragments/albums", url.Values{"title": {tc.title}})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("a refusal is still a rendered list, so 200; got %d for %q", resp.StatusCode, tc.title)
			continue
		}
		body := adminBody(t, resp)
		if !strings.Contains(body, tc.want) {
			t.Errorf("refusing %q should say %q\n%s", tc.title, tc.want, body)
		}
		// The list survives the refusal — that is the reason this is rendered into it.
		if !strings.Contains(body, "Natten") {
			t.Errorf("the list must still be there after a refusal\n%s", body)
		}
	}

	if got := len(pub.Messages); got != 0 {
		t.Errorf("a refused title must publish nothing, got %d events", got)
	}
}

// **The shell triggers on load; the fragment must not.**
//
// This is the one way the pattern bites. htmx processes the attributes of what it swaps in, so a fragment that
// carried `hx-trigger="load"` would re-fetch itself the moment it arrived — forever, a request loop against an
// endpoint that reads the database. It is not visible in a browser beyond a busy network tab, so it is worth a
// test rather than a comment.
func TestTheAlbumListFragmentDoesNotRetriggerItself(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator())

	// Read as the trigger's *events* rather than by searching for the literal `hx-trigger="load"`, because a
	// trigger list is comma-separated: `load, albums-changed from:body` would sail past that needle and loop in
	// exactly the same way. A guard a plausible mistake slips through is worse than no guard.
	trigger := albumListTrigger(t, albumListFragment(t, srv))
	for _, ev := range strings.Split(trigger, ",") {
		// The event name is the first word; `from:body` and the other modifiers follow it.
		if fields := strings.Fields(ev); len(fields) > 0 && fields[0] == "load" {
			t.Errorf("the returned fragment must not trigger on load, or it re-fetches forever: %q", trigger)
		}
	}

	// The page's shell is where `load` belongs, and it must target the same id the fragment re-states.
	page := adminAsset(t, "page.html")
	shell := page[strings.Index(page, `<section id="albums"`):]
	shell = shell[:strings.Index(shell, "</section>")]
	for _, want := range []string{`hx-get="/admin/fragments/albums"`, `hx-trigger="load"`, `hx-swap="outerHTML"`} {
		if !strings.Contains(shell, want) {
			t.Errorf("the album list shell needs %s\n%s", want, shell)
		}
	}
}

// The list re-fetches when something **else** on the page creates an album.
//
// The add-to-album sheet can create one, because a curator creates an album in order to file a selection into it.
// The list above would then be stale by one album. It is told rather than redrawn: the sheet fires
// `albums-changed` on `body`, and the fragment listens for it. That one event is the whole coupling left between
// the script and the list.
func TestTheAlbumListFragmentListensForAlbumsChanged(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator())

	if trigger := albumListTrigger(t, albumListFragment(t, srv)); !strings.Contains(trigger, "albums-changed from:body") {
		t.Errorf("the fragment must re-fetch when the page says the albums changed, got %q", trigger)
	}
	if js := adminAsset(t, "page.js"); !strings.Contains(js, "new Event('albums-changed')") {
		t.Error("creating an album from the add-to-album sheet must tell the list, or it goes stale by one album")
	}
}

// Without the album read model the fragment says so rather than rendering an empty list.
//
// An empty list and an unavailable one look identical, and the difference matters: one means "create an album",
// the other means "wait".
func TestTheAlbumListFragmentSaysWhenTheAlbumsAreUnavailable(t *testing.T) {
	_, srv := adminApp(t)

	resp := getAdmin(t, srv, "/admin/fragments/albums", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503 with no album read model, got %d", resp.StatusCode)
	}
}

// The fragments sit behind the admin credential like everything else on this surface.
//
// `TestAdminRoutesUseOnlyTheAdminWrapper` already reads the routing table for this, but these are the first
// `/admin/*` routes that *write*, so it is worth one request that proves an anonymous caller cannot.
func TestTheAlbumListFragmentsRequireTheAdminCredential(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1"}}))

	if resp := getAdmin(t, srv, "/admin/fragments/albums", "", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401 without the credential, got %d", resp.StatusCode)
	}

	for _, path := range []string{"/admin/fragments/albums", "/admin/fragments/albums/al-1/published"} {
		req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader("published=true&title=x"))
		if err != nil {
			t.Fatalf("building the request: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-Proto", "https")
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("want 401 for an anonymous POST %s, got %d", path, resp.StatusCode)
		}
	}
	if got := len(pub.Messages); got != 0 {
		t.Errorf("an unauthenticated request must publish nothing, got %d events", got)
	}
}

// albumListTrigger returns the fragment's `hx-trigger` attribute value.
func albumListTrigger(t *testing.T, body string) string {
	t.Helper()
	const attr = `hx-trigger="`
	i := strings.Index(body, attr)
	if i < 0 {
		t.Fatalf("the fragment has no hx-trigger, so nothing refreshes it\n%s", body)
	}
	rest := body[i+len(attr):]
	return rest[:strings.Index(rest, `"`)]
}
