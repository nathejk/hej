package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/photo"
	"nathejk.dk/nathejk/table/publicpatrol"
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
// `/{year}/album/{slug}/edit` — where publish, the title, the order and the captions live — had no link from anywhere
// and was reachable only by typing a slug into the address bar. The publish half of PRD 022 §5 was built and
// unusable. A feature with no route to it is indistinguishable from a missing one.
func TestTheAlbumListFragmentLinksEachAlbumToItsEditor(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "natten", Title: "Natten", Published: true, ItemCount: 4,
	}}))

	body := albumListFragment(t, srv)

	if !strings.Contains(body, `href="/2026/album/natten/edit"`) {
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
	// In main.js since the split (task 395): the script announces on `body` and the fragment decides whether to
	// care, which is the one piece of coupling between the two and deliberately one-way.
	if js := adminAsset(t, "main.js"); !strings.Contains(js, "new Event('albums-changed')") {
		t.Error("something must tell the list when the albums change, or it goes stale by one album")
	}
	// And a feature that changes the albums must actually call it. The add-to-album sheet is the case that
	// prompted this: a curator creating an album there then looks for it in the list above.
	if js := adminAsset(t, "albumaction.js"); !strings.Contains(js, "ctx.albumsChanged()") {
		t.Error("filing a selection into albums changes their item counts; the list must be told")
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

// ---------------------------------------------------------------------------
// The action sheets' pickers (steps 4 and 5 of task 395).
//
// Four more fetch-then-render blocks left page.js. Each carried a rule that only a grep could hold; each is now a
// response an ordinary HTTP test reads.
//
// The division of labour these assert is the one the remaining sheets should follow: **the fragment renders what
// the server knows, the browser keeps what only it knows.** The selection is the clearest case — it is a Set in
// page.js, no fragment goes looking for it, and so the sentence counting it stays client-side while the sentence
// about what albums exist does not.
// ---------------------------------------------------------------------------

// The add-to-album picker offers live albums, marks drafts, and does not offer a deleted one.
//
// Deleted albums are filtered here and **shown** by the list on the page. Both are right, and the difference is
// the point: the list answers "what have I got", this answers "where can this go", and a deleted album is not
// somewhere a photograph can be filed.
func TestTheAlbumPickerOffersLiveAlbumsOnly(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Slug: "udgivet", Title: "Udgivet album", Published: true, ItemCount: 4}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-2", Slug: "kladde", Title: "Kladde album", ItemCount: 1}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-3", Slug: "slettet", Title: "Slettet album", Deleted: true}},
	))

	body := adminBody(t, postAdminForm(t, srv, "/admin/fragments/albumpicker", nil))

	if !strings.Contains(body, `value="al-1"`) || !strings.Contains(body, `value="al-2"`) {
		t.Errorf("both live albums must be offered\n%s", body)
	}
	if strings.Contains(body, `value="al-3"`) {
		t.Error("a deleted album must not be offered: a photograph cannot be filed into one")
	}
	// A draft is marked, because "why is it not on the frontpage" is the question a curator asks after filing
	// forty photographs into an album they never published.
	if !strings.Contains(body, `<span class="draft">kladde</span>`) {
		t.Errorf("a draft must be marked in the picker\n%s", body)
	}
	if !strings.Contains(body, "1 billede<") || !strings.Contains(body, "4 billeder") {
		t.Errorf("the item counts need their singular and plural; see task 387\n%s", body)
	}
	// Nothing is ticked on a plain open: the boxes are about this selection, and a previous one's choices are not
	// a statement about it.
	if strings.Contains(body, "checked") {
		t.Errorf("a plain open must tick nothing\n%s", body)
	}
}

// The picker is empty-stated by the server, because only the server knows the albums.
//
// The sheet's other sentence — how many photographs are selected — stays in the browser, because only the browser
// knows that. That split is the convention, and this is the half of it that is testable.
func TestTheAlbumPickerSaysWhenThereAreNoAlbums(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator())

	body := adminBody(t, postAdminForm(t, srv, "/admin/fragments/albumpicker", nil))
	if !strings.Contains(body, "Der er ingen album endnu. Opret et nedenfor.") {
		t.Errorf("an empty picker must say so and point at the form below it\n%s", body)
	}
}

// **Creating an album from the sheet keeps the boxes that were already ticked, and ticks the new one.**
//
// Ticking the new one is the behaviour that was there before: creating an album here is something a curator does
// *in order to* file the current selection into it. Keeping the others is new — the old version rebuilt the list
// from scratch and silently dropped them, which is the sort of thing nobody reports and everybody works around.
func TestCreatingAnAlbumFromThePickerKeepsTheTicksAndAddsTheNewOne(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Slug: "en", Title: "Et album", Published: true}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-2", Slug: "to", Title: "To album", Published: true}},
	))

	resp := postAdminForm(t, srv, "/admin/fragments/albumpicker/albums", url.Values{
		"title":    {"Lørdag morgen"},
		"albumIds": {"al-2"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	// Read before the body: the header says whether the page's album list needs re-fetching.
	if got := resp.Header.Get("HX-Trigger"); got != "albums-changed" {
		t.Errorf("a create must tell the page's album list to re-fetch, got HX-Trigger %q", got)
	}
	body := adminBody(t, resp)

	if got := len(pub.Messages); got != 1 {
		t.Fatalf("want 1 event, got %d", got)
	}
	var created album.Created
	if err := pub.Messages[0].Body(&created); err != nil {
		t.Fatalf("decoding the event: %v", err)
	}
	// The same shared helper the JSON endpoint uses, so the slug folds the same way.
	if created.Slug != "loerdag-morgen" {
		t.Errorf("want the shared slugify, got %q", created.Slug)
	}

	// al-2 was ticked and stays ticked; al-1 was not and does not become so.
	if !strings.Contains(body, `value="al-2" checked`) {
		t.Errorf("a box that was ticked must survive the create\n%s", body)
	}
	if strings.Contains(body, `value="al-1" checked`) {
		t.Error("a box that was not ticked must not become ticked")
	}
	if !strings.Contains(body, "er oprettet som kladde og valgt") {
		t.Errorf("the outcome must say the album is a draft and that it was chosen\n%s", body)
	}
}

// A refused title leaves the ticks alone and says why, rather than costing the curator their choices.
func TestARefusedCreateInThePickerKeepsTheTicks(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", Published: true}},
	))

	for _, tc := range []struct{ title, want string }{
		{"", "skal have en titel"},
		{"Natten", "“natten”"},
	} {
		resp := postAdminForm(t, srv, "/admin/fragments/albumpicker/albums", url.Values{
			"title":    {tc.title},
			"albumIds": {"al-1"},
		})
		body := adminBody(t, resp)
		if !strings.Contains(body, tc.want) {
			t.Errorf("refusing %q should say %q\n%s", tc.title, tc.want, body)
		}
		if !strings.Contains(body, `value="al-1" checked`) {
			t.Errorf("a refusal must not cost the curator their ticks\n%s", body)
		}
		// And nothing tells the page's list to re-fetch, because nothing was created.
		if got := resp.Header.Get("HX-Trigger"); got != "" {
			t.Errorf("a refusal must not claim the albums changed, got HX-Trigger %q", got)
		}
	}
	if got := len(pub.Messages); got != 0 {
		t.Errorf("a refused title must publish nothing, got %d events", got)
	}
}

// The delete sheet's picker offers live albums with drafts marked, and no deleted one.
//
// You cannot remove a photograph from an album that is gone, so offering one would be a row that can only 404.
func TestTheDeleteSheetsAlbumPickerOffersLiveAlbumsOnly(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Slug: "udgivet", Title: "Udgivet album", Published: true}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-2", Slug: "kladde", Title: "Kladde album"}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-3", Slug: "slettet", Title: "Slettet album", Deleted: true}},
	))

	resp := getAdmin(t, srv, "/admin/fragments/delalbumpicker", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	body := adminBody(t, resp)

	if strings.Contains(body, `value="al-3"`) {
		t.Error("a deleted album must not be offered as somewhere to remove from")
	}
	if !strings.Contains(body, "Kladde album (kladde)") {
		t.Errorf("a draft must be marked in the select, where there is no room for a badge\n%s", body)
	}
	// The empty option stays, so opening the sheet never pre-selects an album — the remove button is enabled by
	// choosing one, and a pre-selected album would be one click from removing from the wrong one.
	if !strings.Contains(body, `<option value="">`) {
		t.Errorf("the select must start on no album\n%s", body)
	}
}

// ---------------------------------------------------------------------------
// The patrol confirmation.
// ---------------------------------------------------------------------------

// patrolFragment asks for the confirmation line for a number.
func patrolFragment(t *testing.T, srv *httptest.Server, number string) string {
	t.Helper()

	resp := getAdmin(t, srv, "/admin/fragments/patrol?number="+url.QueryEscape(number),
		testAdminUser, testAdminPass)
	// **Always 200, including "no such patrol".** htmx does not swap a 4xx body by default, so a 404 here would
	// leave the previous confirmation on screen beside a changed number — precisely the disagreement the
	// confirmation exists to prevent. The JSON endpoint keeps its 404, because a status code is what a JSON
	// client reads.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("every lookup outcome must be a rendered line, got %d for %q", resp.StatusCode, number)
	}
	return adminBody(t, resp)
}

// The confirmation names the patrol, its group and its korps — and **never a person**.
//
// The whole design of the tag follows from `publicpatrol.Queries` having no read that lists patrols and no field
// for a person: that absence is deliberate, because a list read is what a scraper would ask for. It is also why
// the curator types the number off the sign rather than picking from a roster.
func TestThePatrolConfirmationNamesThePatrolAndNeverAPerson(t *testing.T) {
	_, srv, _ := tagApp(t, &stubPublicPatrols{
		byNumber: map[string]publicpatrol.Patrol{"42": oernene()},
	})

	body := patrolFragment(t, srv, "42")

	for _, want := range []string{"Patrulje 42", "Ørnene", "1. Søllerød Gruppe"} {
		if !strings.Contains(body, want) {
			t.Errorf("the confirmation must contain %q, or there is nothing to confirm against\n%s", want, body)
		}
	}
	// The number is what the tag button sends — not the resolved team id, which the server re-resolves so a
	// client cannot tag a patrol other than the one the curator was shown.
	if !strings.Contains(body, `data-number="42"`) {
		t.Errorf("the confirmed number must be carried for the tag button\n%s", body)
	}
	if strings.Contains(body, "team-9") {
		t.Error("the team id must not reach the page: the server re-resolves the number when tagging")
	}
	// `ok` is what styles the line as a confirmation rather than a message.
	if !strings.Contains(body, "ok") {
		t.Errorf("a found patrol must read as a confirmation\n%s", body)
	}
}

// A lookup that finds nothing carries **no number**, so the tag button cannot act on it.
//
// This is the one way this feature could tag the wrong patrol: a stale confirmation left beside a changed number.
// The absence of `data-number` is what makes that unexpressible rather than merely unlikely.
func TestAFailedPatrolLookupLeavesNoConfirmation(t *testing.T) {
	_, srv, _ := tagApp(t, &stubPublicPatrols{
		byNumber: map[string]publicpatrol.Patrol{"42": oernene()},
	})

	for _, tc := range []struct{ number, want string }{
		{"", "Skriv patruljens nummer."},
		{"abc", "skal være et tal"},
		{"99", "Der er ingen patrulje med nummer 99 i år."},
	} {
		body := patrolFragment(t, srv, tc.number)
		if !strings.Contains(body, tc.want) {
			t.Errorf("looking up %q should say %q\n%s", tc.number, tc.want, body)
		}
		if strings.Contains(body, "data-number") {
			t.Errorf("looking up %q must leave no confirmation for the tag button\n%s", tc.number, body)
		}
	}
}

// "042" and "42" are one patrol, because that is what the number on the sign means.
//
// The same normalisation the public patrol page applies, reused rather than reimplemented — and worth a test here
// because a fragment that normalised differently from the tag endpoint would show one patrol and tag another.
func TestThePatrolConfirmationNormalisesTheNumberTheSameWayTheTagDoes(t *testing.T) {
	patrols := &stubPublicPatrols{byNumber: map[string]publicpatrol.Patrol{"42": oernene()}}
	_, srv, _ := tagApp(t, patrols)

	body := patrolFragment(t, srv, "042")
	if !strings.Contains(body, `data-number="42"`) {
		t.Errorf("a leading zero must not make a different patrol\n%s", body)
	}
	if len(patrols.asked) == 0 || patrols.asked[0] != "42" {
		t.Errorf("the read must be asked for the normalised number, got %v", patrols.asked)
	}
}

// A broken read is a sentence, not a stack trace — and still carries no confirmation.
func TestAPatrolLookupFailureSaysSoWithoutConfirmingAnything(t *testing.T) {
	_, srv, _ := tagApp(t, &stubPublicPatrols{err: errPatrolReadFailed})

	body := patrolFragment(t, srv, "42")
	if !strings.Contains(body, "Kunne ikke søge") {
		t.Errorf("a failed read needs a Danish sentence\n%s", body)
	}
	if strings.Contains(body, "data-number") {
		t.Error("a failed read must not leave a confirmation behind")
	}
}

// ---------------------------------------------------------------------------
// The position sheet's post picker (step 5).
// ---------------------------------------------------------------------------

// The picker offers the sited posts in route order, carrying the id and the coordinates.
//
// The **id** is the option's value; the coordinates are `data-` attributes and exist only to move the map's pin. A
// stale coordinate in this browser must not be able to become a pin on a public map, so the save sends the id and
// the server resolves it.
func TestTheCheckpointPickerFragmentOffersTheSitedPosts(t *testing.T) {
	_, srv, _ := positionApp(t)

	resp := getAdmin(t, srv, "/admin/fragments/checkpointpicker", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	body := adminBody(t, resp)

	if !strings.Contains(body, "Post 3") {
		t.Errorf("the sited posts must be offered\n%s", body)
	}
	if !strings.Contains(body, "data-lat=") || !strings.Contains(body, "data-lng=") {
		t.Errorf("each post needs its coordinates, for the map's pin\n%s", body)
	}
	// The wrapper is the swap target, and it has to be in the response or a re-render orphans the empty-state
	// sentence beside the new select.
	if !strings.Contains(body, `id="cppickwrap"`) {
		t.Errorf("the fragment must render its own swap target\n%s", body)
	}
	// Nothing is pre-selected: the sheet opens with neither a post nor a map click chosen, which is what keeps
	// the save button disabled until the curator says something.
	if !strings.Contains(body, `<option value="">`) {
		t.Errorf("the picker must start on no post\n%s", body)
	}
}

// **No sited posts gets a sentence, and it is the one that connects the two symptoms.**
//
// An empty course is the ordinary early-season state. It is also exactly the condition that makes a
// curator-placed point unjudgeable — the bounds check has no race area to judge against, so the verdict is
// `unknown` and the photograph does not reach the public map. Explaining those together is the difference between
// a curator understanding the tool and filing a bug.
func TestTheCheckpointPickerFragmentExplainsAnUnsitedCourse(t *testing.T) {
	app, srv, _ := positionApp(t)
	app.models.CheckpointCurator = stubCheckpointCurator{}

	body := adminBody(t, getAdmin(t, srv, "/admin/fragments/checkpointpicker", testAdminUser, testAdminPass))

	if !strings.Contains(body, "Ingen poster har en placering endnu") {
		t.Errorf("an unsited course must say so\n%s", body)
	}
	// And it must say what follows from it, not merely that the list is empty.
	if !strings.Contains(body, "kan ikke vurderes") {
		t.Errorf("the consequence for the position's verdict must be stated too\n%s", body)
	}
	// The picker still renders, because clicking the map is still a way to do the job.
	if !strings.Contains(body, `<select id="cppick">`) {
		t.Errorf("the select must render even with nothing in it\n%s", body)
	}
}

// Every sheet fragment sits behind the admin credential.
func TestTheSheetFragmentsRequireTheAdminCredential(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator())

	for _, path := range []string{
		"/admin/fragments/delalbumpicker",
		"/admin/fragments/patrol?number=42",
		"/admin/fragments/checkpointpicker",
	} {
		if resp := getAdmin(t, srv, path, "", ""); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("want 401 for an anonymous GET %s, got %d", path, resp.StatusCode)
		}
	}
}

// ---------------------------------------------------------------------------
// The contact sheet (step 6 of task 395).
//
// # The risk the task flagged, and what these assert instead
//
// Task 395 called this the one real risk: PRD 022 §7 requires the selection to survive every action, and "a naive
// hx-swap over the grid destroys the selection set".
//
// It does not, and the reason is worth keeping written down. **The selection was never in the grid.** It is a Set
// of ids in page.js, held outside the DOM deliberately — the file's own comment says a DOM-derived selection
// "would be lost by any re-render, and it would silently shrink to what is currently loaded". A swap replaces
// cells; the Set does not notice.
//
// What that leaves for these tests is the half the server now owns: the cells and their marks. The half it does
// not own is asserted from the other side — the cells arrive `aria-selected="false"` because the server cannot
// know the selection, and page.js re-paints them after every swap.
// ---------------------------------------------------------------------------

// contactSheetFragment asks for a page of thumbnails.
func contactSheetFragment(t *testing.T, srv *httptest.Server, query string) string {
	t.Helper()

	path := "/admin/fragments/photos"
	if query != "" {
		path += "?" + query
	}
	resp := getAdmin(t, srv, path, testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 from the contact sheet fragment, got %d", resp.StatusCode)
	}
	return adminBody(t, resp)
}

// libraryWithMarks is one photograph of each state the cells have to distinguish.
func libraryWithMarks() *libraryCurator {
	return &libraryCurator{
		counts: photo.Counts{Total: 5},
		rows: []photo.LibraryPhoto{
			{ID: photoID("a"), BoundsVerdict: "inside", AlbumCount: 1},
			{ID: photoID("b"), BoundsVerdict: "outside", AlbumCount: 3, TagCount: 2},
			{ID: photoID("c"), BoundsVerdict: "unknown"},
			{ID: photoID("d"), BoundsVerdict: "none"},
			{ID: photoID("e"), BoundsVerdict: "inside", Deleted: true},
		},
	}
}

// **The four position states read differently, and `unknown` must not read as a rejection.**
//
// PRD 011 §6 requires an out-of-bounds coordinate to be visible *as rejected*. `unknown` is a statement about us —
// there was no race area to judge against — so blaming the photograph for it would be wrong. And `none` gets no
// mark at all, because having no coordinate is the ordinary case and a badge for it would be noise on most of the
// sheet.
func TestTheContactSheetDistinguishesTheFourPositionStates(t *testing.T) {
	_, srv := libraryApp(t, libraryWithMarks())

	body := contactSheetFragment(t, srv, "")

	for _, want := range []struct{ needle, why string }{
		{`<span class="mark inside">position</span>`, "a photograph inside the area says so"},
		{`<span class="mark outside">uden for området</span>`, "an out-of-bounds coordinate must read as rejected"},
		{`<span class="mark unknown">ikke vurderet</span>`, "unknown is about us, and must not read as a rejection"},
	} {
		if !strings.Contains(body, want.needle) {
			t.Errorf("the cells are missing %q: %s\n%s", want.needle, want.why, body)
		}
	}

	// The three classes are distinct, which is how this distinction usually dies — one of them quietly sharing
	// another's styling.
	for _, class := range []string{"mark inside", "mark outside", "mark unknown"} {
		if strings.Count(body, class) == 0 {
			t.Errorf("no cell carries %q", class)
		}
	}

	// `none` gets nothing. Checked by counting marks on that one cell rather than by the absence of a string,
	// because there is no string for "no mark" to look for.
	cell := body[strings.Index(body, photoID("d")):]
	cell = cell[:strings.Index(cell, "</button>")]
	// `class="mark` alone would match the `marks` container every cell has, which is how the first version of
	// this test failed against correct output.
	if strings.Contains(cell, `class="mark "`) || strings.Contains(cell, `class="mark"`) ||
		strings.Contains(cell, `class="mark inside"`) || strings.Contains(cell, `class="mark outside"`) ||
		strings.Contains(cell, `class="mark unknown"`) {
		t.Errorf("a photograph with no coordinate must get no position mark\n%s", cell)
	}
}

// The other three marks: how many albums, whether it is tagged, and whether it is deleted.
func TestTheContactSheetMarksAlbumsTagsAndDeletion(t *testing.T) {
	_, srv := libraryApp(t, libraryWithMarks())

	body := contactSheetFragment(t, srv, "")

	if !strings.Contains(body, ">1 album<") {
		t.Errorf("one album needs the singular; see task 387\n%s", body)
	}
	if !strings.Contains(body, ">3 album<") {
		t.Error("the album count must be rendered, so the curator can see what is already sorted")
	}
	if !strings.Contains(body, ">patrulje<") {
		t.Error("a tagged photograph must be marked")
	}
	// A deleted photograph is marked *and* dimmed by a class on the cell, because a badge alone is easy to miss
	// in a grid of 120.
	if !strings.Contains(body, ">slettet<") || !strings.Contains(body, `class="cell gone"`) {
		t.Errorf("a deleted photograph must be marked and dimmed\n%s", body)
	}
}

// The thumbnail is addressed by **id and variant, never by a blob ref**.
//
// The server resolves it through the projection, which is what stops the media route being a file server for the
// whole blob store — if a ref reached the page, the client would have no reason to go through the check.
func TestTheContactSheetAddressesThumbnailsThroughTheProjection(t *testing.T) {
	_, srv := libraryApp(t, libraryWithMarks())

	body := contactSheetFragment(t, srv, "")

	if !strings.Contains(body, `src="/api/admin/photos/`+photoID("a")+`/media?variant=thumb"`) {
		t.Errorf("thumbnails must be addressed by id and variant\n%s", body)
	}
	// Lazy, because a contact sheet is 120 images and eager loading them is how this page becomes unusable on a
	// hotel connection.
	if !strings.Contains(body, `loading="lazy"`) {
		t.Error("thumbnails must load lazily")
	}
}

// **Every cell arrives unselected, because the server does not know the selection.**
//
// This is the half of the contract the fragment cannot fulfil, asserted from the side that can. The selection is a
// Set in page.js and no fragment goes looking for it; the browser re-paints `aria-selected` after every swap, which
// is what lets the selection survive a filter change — a curator narrows to "uden position", picks forty, widens
// to check something, and must still have the forty.
func TestTheContactSheetRendersEveryCellUnselected(t *testing.T) {
	_, srv := libraryApp(t, libraryWithMarks())

	body := contactSheetFragment(t, srv, "")

	if strings.Contains(body, `aria-selected="true"`) {
		t.Error("the server must not claim anything is selected: it cannot know, and guessing would make the " +
			"grid and the action bar disagree about what an action applies to")
	}
	if got := strings.Count(body, `aria-selected="false"`); got != 5 {
		t.Errorf("every cell needs the attribute for the browser to paint, got %d of 5", got)
	}
	// And the paint really is the browser's job.
	if js := adminAsset(t, "contactsheet.js"); !strings.Contains(js, "htmx:afterSwap") ||
		!strings.Contains(js, "for (const cell of sheet.querySelectorAll('.cell')) paint(cell);") {
		t.Error("page.js must re-paint the selection after every swap, or a filter change loses it visually " +
			"while the action bar still counts it")
	}
}

// The shown-count, the "more" button and the header's counts arrive with the cells, out of band.
//
// One request rather than four, because they are answers to the same question: a second request for the counts
// could return a different read.
func TestTheContactSheetCarriesItsSurroundingsOutOfBand(t *testing.T) {
	rows := make([]photo.LibraryPhoto, 7)
	for i := range rows {
		rows[i] = photo.LibraryPhoto{ID: photoID(string(rune('a' + i))), BoundsVerdict: "none"}
	}
	_, srv := libraryApp(t, &libraryCurator{rows: rows, counts: photo.Counts{Total: 7}})

	// A page of 3 out of 7: there is more.
	body := contactSheetFragment(t, srv, "limit=3")
	if !strings.Contains(body, `<p id="sheetnote" class="todo" aria-live="polite" hx-swap-oob="true">3 vist — der er flere</p>`) {
		t.Errorf("the shown-count must say how many and that there are more\n%s", body)
	}
	// **The next offset comes from the server.** The browser adding up page sizes would drift the first time a
	// clamped limit differed from the one it asked for.
	if !strings.Contains(body, `id="more" data-offset="3"`) {
		t.Errorf("the more button must carry where the next page starts\n%s", body)
	}

	// The last page: the button is gone rather than left promising a page that does not exist.
	last := contactSheetFragment(t, srv, "limit=3&offset=6")
	if strings.Contains(last, `id="more"`) {
		t.Errorf("the last page must not offer more\n%s", last)
	}
	// The count is the running total, not this page's size \u2014 it is read as "how far through am I".
	if !strings.Contains(last, ">7 vist<") {
		t.Errorf("the shown-count must be the running total, got\n%s", last)
	}
	// The wrapper is still rendered, so the button's absence is a swap rather than a stale element left behind —
	// and it comes back **exactly empty**, with no whitespace, because `#morewrap:empty` is what collapses the
	// line of space it would otherwise hold open under the grid.
	if !strings.Contains(last, `<p id="morewrap" hx-swap-oob="true"></p>`) {
		t.Errorf("the more button's wrapper must always be swapped, and be empty on the last page\n%s", last)
	}
}

// An empty result is a sentence, not a blank grid.
//
// An empty grid under a filter is an ordinary answer; without saying so it is indistinguishable from one that is
// still loading.
func TestTheContactSheetSaysWhenNothingMatches(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{})

	body := contactSheetFragment(t, srv, "album=none")
	if !strings.Contains(body, "Ingen billeder matcher.") {
		t.Errorf("an empty result must say so\n%s", body)
	}
	if strings.Contains(body, `class="cell`) {
		t.Error("an empty result must render no cells")
	}
}

// **The fragment and the JSON list read one filter through one function.**
//
// The grid renders a page of a filter and "select all matching this filter" pages the ids out of the JSON endpoint.
// PRD 022 §3 says "these forty are from Post 3" is the true shape of the work, and §6 requires selecting beyond
// the loaded page — so two interpretations of one filter is exactly how a bulk action lands on photographs the
// curator never saw.
//
// Asserted by asking both for the same filter and comparing what the read was *asked*, which is the only place the
// two could diverge.
func TestTheContactSheetAndTheJSONListInterpretAFilterIdentically(t *testing.T) {
	curator := libraryWithMarks()
	_, srv := libraryApp(t, curator)

	const q = "album=none&location=no&verdict=unknown&tagged=no&deleted=1"
	contactSheetFragment(t, srv, q)
	if resp := getAdmin(t, srv, "/api/admin/photos?"+q, testAdminUser, testAdminPass); resp.StatusCode != http.StatusOK {
		t.Fatalf("the JSON list refused the same filter: %d", resp.StatusCode)
	}

	if len(curator.filters) != 2 {
		t.Fatalf("want both reads recorded, got %d", len(curator.filters))
	}
	// Compared as rendered text, not with `!=`: `photo.Filter` holds `*bool` for the tri-state filters, so
	// comparing the structs compares pointer addresses and passes for any two reads whatsoever. The first version
	// of this test did exactly that and failed against identical filters.
	if a, b := describeFilter(curator.filters[0]), describeFilter(curator.filters[1]); a != b {
		t.Errorf("the two surfaces translated one filter differently:\n  fragment: %s\n  json:     %s", a, b)
	}
}

// describeFilter renders a filter by value, including through its pointers.
func describeFilter(f photo.Filter) string {
	tri := func(p *bool) string {
		if p == nil {
			return "unset"
		}
		return fmt.Sprintf("%t", *p)
	}
	return fmt.Sprintf("inNoAlbum=%t hasLocation=%s verdict=%q tagged=%s includeDeleted=%t",
		f.InNoAlbum, tri(f.HasLocation), f.Verdict, tri(f.Tagged), f.IncludeDeleted)
}

// An unrecognised filter value is refused by the fragment too, rather than quietly widening the page.
//
// A 400 does not swap in htmx, so the grid keeps the page it had and page.js writes the sentence — which is the
// right outcome: the alternative is a grid silently showing everything while the filter button looks pressed.
func TestTheContactSheetRefusesAnUnrecognisedFilter(t *testing.T) {
	_, srv := libraryApp(t, libraryWithMarks())

	resp := getAdmin(t, srv, "/admin/fragments/photos?verdict=sideways", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 for an unrecognised filter value, got %d", resp.StatusCode)
	}
}

// The grid's own shell stays in the page, with `load` on it and not on the fragment.
//
// The first page cannot be fetched from page.js: htmx is a deferred script and page.js is inline, so `window.htmx`
// does not exist yet while that block runs. A call there would silently do nothing and leave an empty grid.
func TestTheContactSheetLoadsFromItsShell(t *testing.T) {
	page := adminAsset(t, "page.html")

	shell := page[strings.Index(page, `<div id="sheet"`):]
	shell = shell[:strings.Index(shell, ">")+1]
	for _, want := range []string{
		`hx-get="/admin/fragments/photos`, `hx-trigger="load"`, `hx-swap="innerHTML"`,
		// The listbox's own ARIA stays on the shell, because the fragment returns bare cells and never this
		// element — a filter change swaps its contents and "Hent flere" appends to them.
		`role="listbox"`, `aria-multiselectable="true"`,
	} {
		if !strings.Contains(shell, want) {
			t.Errorf("the contact sheet's shell needs %s\n%s", want, shell)
		}
	}

	// And nothing must try to fetch the first page from the script, which is the mistake this arrangement
	// prevents. main.js is where such a call would go, since it is what runs on load.
	if strings.Contains(adminAsset(t, "main.js"), "reloadSheet()") {
		t.Error("the first load must be declared on the shell, not called from main.js: window.htmx does not " +
			"exist yet when that runs, so the call would silently do nothing and leave an empty grid")
	}
}

// Deleting an album from the list (task 396) publishes the same `album.Deleted` the in-app takedown does, and
// says in the note that the photographs survive.
func TestDeletingAnAlbumFromTheListPublishesDeleted(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "natten", Title: "Natten", Published: true, ItemCount: 4,
	}}))

	resp := postAdminForm(t, srv, "/admin/fragments/albums/al-1/deleted", url.Values{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	body := adminBody(t, resp)

	if got := len(pub.Messages); got != 1 {
		t.Fatalf("want exactly 1 event, got %d", got)
	}
	if !strings.Contains(pub.Subjects()[0], ".album.al-1.deleted") {
		t.Errorf("unexpected subject %q", pub.Subjects()[0])
	}
	var payload album.Deleted
	if err := pub.Messages[0].Body(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.AlbumID != "al-1" || payload.Year != "2026" {
		t.Errorf("the event must name the album and the year, got %+v", payload)
	}
	if !strings.Contains(body, "Billederne ligger stadig i arkivet") {
		t.Errorf("the note must say the photographs survive\n%s", body)
	}
}

// An album already deleted is answered with the list, not with a second event.
func TestDeletingADeletedAlbumAppendsNothing(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "natten", Title: "Natten", Deleted: true,
	}}))

	resp := postAdminForm(t, srv, "/admin/fragments/albums/al-1/deleted", url.Values{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if got := len(pub.Messages); got != 0 {
		t.Errorf("want no event for an album already deleted, got %d", got)
	}
}

func TestDeletingAnUnknownAlbumIs404(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator())

	if got := postAdminForm(t, srv, "/admin/fragments/albums/al-nope/deleted", url.Values{}).StatusCode; got != http.StatusNotFound {
		t.Errorf("want 404, got %d", got)
	}
	if len(pub.Messages) != 0 {
		t.Error("an unknown album must not append to the log")
	}
}

// **The delete is behind a confirm that names the album and says the photographs stay.** A deleted album's button
// is gone, since there is nothing left to delete.
func TestTheAlbumDeleteButtonAsksFirst(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", Published: true}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-2", Slug: "vaek", Title: "Væk", Deleted: true}},
	))

	body := albumListFragment(t, srv)

	if !strings.Contains(body, `hx-post="/admin/fragments/albums/al-1/deleted"`) {
		t.Fatalf("a live album should have a delete button\n%s", body)
	}
	i := strings.Index(body, `hx-post="/admin/fragments/albums/al-1/deleted"`)
	button := body[i:]
	button = button[:strings.Index(button, ">")]
	for _, want := range []string{"hx-confirm=", "Natten", "taget af forsiden", "Billederne bliver liggende i arkivet"} {
		if !strings.Contains(button, want) {
			t.Errorf("the confirm should contain %q; got %s", want, button)
		}
	}
	if strings.Contains(body, "/admin/fragments/albums/al-2/deleted") {
		t.Error("a deleted album must not offer delete again")
	}
}
