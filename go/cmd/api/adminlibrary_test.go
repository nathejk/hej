package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/photo"
)

// The contact sheet's reads (PRD 022 §7, §8.4, task 374).

// libraryCurator is a stub library with rows, for the list and media reads.
//
// Richer than `stubPhotoCurator` (adminupload_test.go) because these tests are about *what comes back*, not
// about whether a row exists. Kept separate rather than growing that one: the upload tests care only about
// presence and deletion, and a stub serving both would have to satisfy two sets of expectations at once.
type libraryCurator struct {
	rows   []photo.LibraryPhoto
	counts photo.Counts
	err    error

	// filters records every filter the list read was asked for, so a test can assert the query string was
	// actually translated rather than inferring it from a result that could match by accident.
	filters []photo.Filter
}

func (c *libraryCurator) Library(_ string, f photo.Filter, limit, offset int) ([]photo.LibraryPhoto, error) {
	c.filters = append(c.filters, f)
	if c.err != nil {
		return nil, c.err
	}
	// Paging applied here so `hasMore` is exercised for real rather than stubbed.
	if offset >= len(c.rows) {
		return nil, nil
	}
	end := offset + limit
	if end > len(c.rows) {
		end = len(c.rows)
	}
	return c.rows[offset:end], nil
}

func (c *libraryCurator) Counts(string) (photo.Counts, error) {
	if c.err != nil {
		return photo.Counts{}, c.err
	}
	return c.counts, nil
}

func (c *libraryCurator) Photo(_, photoID string) (photo.LibraryPhoto, bool, error) {
	if c.err != nil {
		return photo.LibraryPhoto{}, false, c.err
	}
	for _, p := range c.rows {
		if p.ID == photoID {
			return p, true, nil
		}
	}
	return photo.LibraryPhoto{}, false, nil
}

func (c *libraryCurator) Tags(string, string) ([]photo.Tag, error) { return nil, c.err }

// libraryApp returns an admin app whose library is the given stub.
func libraryApp(t *testing.T, curator *libraryCurator) (*application, *httptest.Server) {
	t.Helper()

	app, srv := adminApp(t)
	app.models.PhotoCurator = curator
	return app, srv
}

func decodeLibrary(t *testing.T, resp *http.Response) adminLibraryResponse {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		body := adminBody(t, resp)
		t.Fatalf("want 200 before decoding a library response, got %d: %s", resp.StatusCode, body)
	}
	var out adminLibraryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding the library response: %v", err)
	}
	return out
}

func libRow(id string, opts ...func(*photo.LibraryPhoto)) photo.LibraryPhoto {
	p := photo.LibraryPhoto{
		ID:            strings.Repeat(id, 64),
		Ref:           strings.Repeat(id, 64),
		ThumbRef:      strings.Repeat("b", 64),
		BoundsVerdict: photo.BoundsNone,
		Width:         1600,
		Height:        1200,
	}
	for _, o := range opts {
		o(&p)
	}
	return p
}

func TestAdminLibraryListsThePage(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{
		rows:   []photo.LibraryPhoto{libRow("a"), libRow("c"), libRow("d")},
		counts: photo.Counts{Total: 3, InNoAlbum: 2, WithLocation: 1, Plottable: 1},
	})

	out := decodeLibrary(t, getAdmin(t, srv, "/api/admin/photos", testAdminUser, testAdminPass))

	if len(out.Photos) != 3 {
		t.Fatalf("want 3 photographs, got %d", len(out.Photos))
	}
	if out.Counts.Total != 3 || out.Counts.InNoAlbum != 2 {
		t.Errorf("the header counts must travel with the page, got %+v", out.Counts)
	}
	if out.HasMore {
		t.Error("three rows in a page of 120 is not more")
	}
}

// The counts come with the page rather than from a second endpoint: they are shown on the same line as the grid
// and must agree with it, and two requests could each be correct about a different instant.
//
// Asserted by reflection rather than by matching the source text — the first version searched for the field
// declaration and broke immediately, because gofmt aligns struct tags with padding that a literal cannot
// predict.
func TestAdminLibraryCountsTravelWithThePage(t *testing.T) {
	var found bool
	for _, f := range structFieldNames(adminLibraryResponse{}) {
		if f == "Counts" {
			found = true
		}
	}
	if !found {
		t.Error("the counts belong on the library response, so the grid and the header cannot disagree")
	}
}

// Paging asks for one row more than the page, so "is there more" costs no second aggregate.
func TestAdminLibraryPagesWithoutACount(t *testing.T) {
	rows := make([]photo.LibraryPhoto, 0, 5)
	for _, c := range []string{"a", "b", "c", "d", "e"} {
		rows = append(rows, libRow(c))
	}
	_, srv := libraryApp(t, &libraryCurator{rows: rows})

	first := decodeLibrary(t, getAdmin(t, srv, "/api/admin/photos?limit=2", testAdminUser, testAdminPass))
	if len(first.Photos) != 2 {
		t.Fatalf("want a page of 2, got %d", len(first.Photos))
	}
	if !first.HasMore {
		t.Error("want hasMore with five rows and a page of two")
	}

	last := decodeLibrary(t, getAdmin(t, srv, "/api/admin/photos?limit=2&offset=4", testAdminUser, testAdminPass))
	if len(last.Photos) != 1 || last.HasMore {
		t.Errorf("the last page should hold one row and report no more, got %d/%v",
			len(last.Photos), last.HasMore)
	}
}

// All six filters PRD 022 §6 names reach the projection, and they compose.
func TestAdminLibraryTranslatesEveryFilter(t *testing.T) {
	yes, no := true, false

	for name, tc := range map[string]struct {
		query string
		want  photo.Filter
	}{
		"not in any album":  {"album=none", photo.Filter{InNoAlbum: true}},
		"without location":  {"location=no", photo.Filter{HasLocation: &no}},
		"with location":     {"location=yes", photo.Filter{HasLocation: &yes}},
		"out of bounds":     {"verdict=outside", photo.Filter{Verdict: photo.BoundsOutside}},
		"not judged":        {"verdict=unknown", photo.Filter{Verdict: photo.BoundsUnknown}},
		"tagged":            {"tagged=yes", photo.Filter{Tagged: &yes}},
		"untagged":          {"tagged=no", photo.Filter{Tagged: &no}},
		"including deleted": {"deleted=1", photo.Filter{IncludeDeleted: true}},
		// The conjunction the bulk-position workflow starts from.
		"unsorted and unplaced": {"album=none&location=no",
			photo.Filter{InNoAlbum: true, HasLocation: &no}},
	} {
		curator := &libraryCurator{}
		_, srv := libraryApp(t, curator)

		getAdmin(t, srv, "/api/admin/photos?"+tc.query, testAdminUser, testAdminPass)

		if len(curator.filters) != 1 {
			t.Fatalf("%s: want one library read, got %d", name, len(curator.filters))
		}
		got := curator.filters[0]

		if got.InNoAlbum != tc.want.InNoAlbum || got.Verdict != tc.want.Verdict ||
			got.IncludeDeleted != tc.want.IncludeDeleted ||
			!boolPtrEqual(got.HasLocation, tc.want.HasLocation) ||
			!boolPtrEqual(got.Tagged, tc.want.Tagged) {
			t.Errorf("%s: filter %q became %+v, want %+v", name, tc.query, got, tc.want)
		}
	}
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// **An unrecognised filter value is refused, not ignored.**
//
// A typo'd `verdict=insid` silently ignored would show the curator everything while they believed they were
// looking at the plottable subset — and the action bar acts on what is selected. The difference between
// refusing and ignoring is the difference between a confusing screen and a bulk edit applied to the wrong
// forty photographs.
func TestAdminLibraryRefusesAnUnknownFilterValue(t *testing.T) {
	curator := &libraryCurator{}
	_, srv := libraryApp(t, curator)

	for _, q := range []string{
		"album=all", "location=maybe", "verdict=insid", "verdict=plottable", "tagged=perhaps",
	} {
		resp := getAdmin(t, srv, "/api/admin/photos?"+q, testAdminUser, testAdminPass)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400 for an unrecognised filter value, got %d", q, resp.StatusCode)
		}
	}
	if len(curator.filters) != 0 {
		t.Errorf("a refused filter must not reach the projection, got %d reads", len(curator.filters))
	}
}

// A malformed `limit` falls back rather than erroring, unlike a malformed filter. The asymmetry is deliberate:
// a page size is cosmetic, while a filter decides what a bulk action applies to.
func TestAdminLibraryToleratesAMalformedLimit(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{rows: []photo.LibraryPhoto{libRow("a")}})

	out := decodeLibrary(t, getAdmin(t, srv, "/api/admin/photos?limit=abc&offset=-5",
		testAdminUser, testAdminPass))
	if out.Limit != 120 || out.Offset != 0 {
		t.Errorf("want the defaults echoed back, got limit=%d offset=%d", out.Limit, out.Offset)
	}
}

// The page size is clamped, and the clamp is echoed so a client can see it rather than wondering why it got 500
// rows after asking for 100000.
func TestAdminLibraryClampsThePageSize(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{})

	out := decodeLibrary(t, getAdmin(t, srv, "/api/admin/photos?limit=100000", testAdminUser, testAdminPass))
	if out.Limit != 500 {
		t.Errorf("want the limit clamped to 500 and echoed, got %d", out.Limit)
	}
}

// The list carries no blob refs. The client addresses bytes by id and variant, which is what keeps the media
// route's projection check load-bearing — a ref in this payload would give the client no reason to go through it.
func TestAdminLibraryReturnsNoBlobRefs(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{rows: []photo.LibraryPhoto{libRow("a")}})

	resp := getAdmin(t, srv, "/api/admin/photos", testAdminUser, testAdminPass)
	body := adminBody(t, resp)

	for _, leak := range []string{"blobRef", "thumbRef", "\"ref\""} {
		if strings.Contains(body, leak) {
			t.Errorf("the library payload carries %q; bytes are addressed by id and variant\ngot: %s",
				leak, body)
		}
	}
}

// The library read must never carry a person. Behind a credential is not a reason to relax it — the rule is
// about what the table is allowed to know (PRD 022 §8.3), and task 381 walks it structurally.
func TestAdminLibraryPayloadHasNowhereToPutAPerson(t *testing.T) {
	for _, field := range structFieldNames(adminLibraryPhoto{}) {
		if isPersonShaped(field) {
			t.Errorf("adminLibraryPhoto gained a person-shaped field %q", field)
		}
	}
}

// **A wire-format bug the Go tests structurally cannot catch.**
//
// `adminCountsView` is rendered by the page template *and* serialised by the list endpoint. Without JSON tags
// the API exposes Go field names (`Total`), while the page's script reads `total` — so the header silently stops
// updating. Nothing failed: the template is indifferent to tags, and the Go tests decode into this same struct,
// so the casing round-trips perfectly. Only a browser, or a look at the live endpoint, notices.
//
// This asserts the wire names directly, against the JSON rather than against the struct, so the two cannot drift
// again.
func TestAdminCountsUseWireNamesTheScriptCanRead(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{counts: photo.Counts{
		Total: 7, InNoAlbum: 6, WithLocation: 5, Plottable: 4,
		OutOfBounds: 3, Unknown: 2, Tagged: 1, Deleted: 8,
	}})

	resp := getAdmin(t, srv, "/api/admin/photos", testAdminUser, testAdminPass)
	body := adminBody(t, resp)

	// Decoded loosely, so the assertion is about the wire and not about the Go type.
	var raw struct {
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	want := map[string]int{
		"total": 7, "inNoAlbum": 6, "withLocation": 5, "plottable": 4,
		"outOfBounds": 3, "unknown": 2, "tagged": 1, "deleted": 8,
	}
	for key, n := range want {
		got, ok := raw.Counts[key]
		if !ok {
			t.Errorf("the counts payload has no %q field; the page's script reads that name", key)
			continue
		}
		if got != n {
			t.Errorf("%s: want %d, got %d", key, n, got)
		}
	}
}

// And **the page and the fragment render every one of them from one definition** (task 395).
//
// This used to assert that the page bound each count by its JSON field name, because the browser refreshed the
// header by reading those names out of the list payload. It no longer does: the counts are a template definition
// rendered by the page on first load and again by the contact sheet's fragment as an out-of-band swap, so there is
// no wire name in the middle to keep in step.
//
// What still has to hold is the reason the old test existed — that **all eight refresh together**. A header
// showing "312 billeder / 47 uden album" with a stale second figure is worse than no figure, because the curator
// uses it to decide what to sort next. One shared definition and one swap is what makes half-stale impossible;
// this asserts both sides really do use it.
func TestTheHeaderCountsComeFromOneDefinition(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{counts: photo.Counts{
		Total: 7, InNoAlbum: 6, WithLocation: 5, Plottable: 4,
		OutOfBounds: 3, Unknown: 2, Tagged: 1, Deleted: 8,
	}})

	page := adminBody(t, getAdmin(t, srv, "/admin", testAdminUser, testAdminPass))
	fragment := adminBody(t, getAdmin(t, srv, "/admin/fragments/photos", testAdminUser, testAdminPass))

	// The eight labels, each beside its number. Matched as the rendered pair rather than as a bare integer, so a
	// count that lost its label — or a label that lost its count — fails here.
	for _, want := range []string{
		"<span class=\"n\">7</span><span class=\"k\">billeder",
		"<span class=\"n\">6</span><span class=\"k\">uden album",
		"<span class=\"n\">5</span><span class=\"k\">med position",
		"<span class=\"n\">4</span><span class=\"k\">på kortet",
		"<span class=\"n\">3</span><span class=\"k\">uden for området",
		"<span class=\"n\">2</span><span class=\"k\">ikke vurderet",
		"<span class=\"n\">1</span><span class=\"k\">med patrulje",
		"<span class=\"n\">8</span><span class=\"k\">slettede",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page's header is missing %q", want)
		}
		if !strings.Contains(fragment, want) {
			t.Errorf("the fragment's header swap is missing %q; it would leave that number stale", want)
		}
	}

	// **One element, swapped out of band.** Eight separate swaps would be eight chances for half the header to be
	// from the previous read.
	if got := strings.Count(fragment, `hx-swap-oob="true"`); got != 3 {
		t.Errorf("want 3 out-of-band swaps (the counts, the shown-count and the more button), got %d", got)
	}
	if !strings.Contains(fragment, `<ul id="counts" class="counts" hx-swap-oob="true">`) {
		t.Errorf("the counts must arrive as one out-of-band element\n%s", fragment)
	}
}

// ---------------------------------------------------------------------------
// The media route.
// ---------------------------------------------------------------------------

// **The most important test on this surface.**
//
// A library photograph's id *is* its content ref, so handing the path segment to the blob store would work
// perfectly for every real photograph — and would also serve every *other* object in the store to anyone
// holding the shared password: a participant's portrait, a glimt somebody took down, a diploma. One password,
// one URL shape, the whole store.
//
// This uploads bytes that exist in the store but belong to no library row, then asks for them by their ref.
func TestAdminMediaRefusesARefThatIsNotARow(t *testing.T) {
	app, srv := libraryApp(t, &libraryCurator{rows: []photo.LibraryPhoto{libRow("a")}})

	// A real object in the store — as a portrait or a glimt's media would be — with no library row naming it.
	orphan, err := app.blobs.Put(context.Background(), []byte("somebody else's photograph"))
	if err != nil {
		t.Fatalf("seeding the blob store: %v", err)
	}

	resp := getAdmin(t, srv, "/api/admin/photos/"+orphan.String()+"/media", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("a valid ref that is not a library row must answer 404, got %d — this route would "+
			"otherwise serve the whole blob store behind one shared password", resp.StatusCode)
	}
}

// A deleted photograph is *found* by the curator read — that is what the interface is for — but its bytes are
// not served. A takedown that still answered on a URL would be a takedown in name only.
//
// No bytes are seeded: the handler must refuse before it ever reaches the store, which this also proves.
func TestAdminMediaRefusesADeletedPhotograph(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{
		rows: []photo.LibraryPhoto{libRow("a", func(p *photo.LibraryPhoto) { p.Deleted = true })},
	})

	resp := getAdmin(t, srv, "/api/admin/photos/"+strings.Repeat("a", 64)+"/media",
		testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("a deleted photograph's bytes must not be served, got %d", resp.StatusCode)
	}
}

// The happy path, and the variant.
//
// The rows are built **from** objects actually in the store rather than from fixture refs, because the store is
// content-addressed: a ref and its bytes cannot be chosen independently. Storing first and reading the refs back
// is the only honest way to have a row whose ref resolves.
func TestAdminMediaServesTheStoredBytes(t *testing.T) {
	app, srv := adminApp(t)

	fullBytes := []byte("the full rendition")
	thumbBytes := []byte("the thumbnail")
	fullRef, err := app.blobs.Put(context.Background(), fullBytes)
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	thumbRef, err := app.blobs.Put(context.Background(), thumbBytes)
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	// The id is the full rendition's ref, exactly as the upload path derives it (PRD 022 §8.5).
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID:            fullRef.String(),
		Ref:           fullRef.String(),
		ThumbRef:      thumbRef.String(),
		BoundsVerdict: photo.BoundsNone,
	}}}

	base := "/api/admin/photos/" + fullRef.String() + "/media"
	for name, tc := range map[string]struct {
		path string
		want string
	}{
		"full":  {base, string(fullBytes)},
		"thumb": {base + "?variant=thumb", string(thumbBytes)},
	} {
		resp := getAdmin(t, srv, tc.path, testAdminUser, testAdminPass)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: want 200, got %d", name, resp.StatusCode)
		}
		if got := adminBody(t, resp); got != tc.want {
			t.Errorf("%s: served %q, want %q", name, got, tc.want)
		}
	}
}

// Even the bytes are unstorable. The photographs are immutable so caching them would be safe in the ordinary
// sense, but PRD 022 §6 requires every admin response to be `no-store`: a contact sheet of the event's
// photographs left in a shared laptop's disk cache outlives the session that fetched it.
func TestAdminMediaIsNotCacheable(t *testing.T) {
	app, srv := adminApp(t)

	ref, err := app.blobs.Put(context.Background(), []byte("a photograph"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID: ref.String(), Ref: ref.String(), BoundsVerdict: photo.BoundsNone,
	}}}

	resp := getAdmin(t, srv, "/api/admin/photos/"+ref.String()+"/media", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("want no-store on admin media, got %q — the public media route's year-long immutable cache "+
			"is the wrong answer here", got)
	}
}

// An unknown id is a 404, and it must look exactly like the orphan-ref case above. Nothing distinguishes "no
// such photograph" from "those bytes exist but are not yours", which is what stops the route being usable to
// probe the store.
func TestAdminMediaRefusesAnUnknownID(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{})

	for _, id := range []string{
		strings.Repeat("f", 64), // ref-shaped, unknown
		"not-a-ref",
		"../../etc/passwd",
	} {
		resp := getAdmin(t, srv, "/api/admin/photos/"+id+"/media", testAdminUser, testAdminPass)
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMovedPermanently {
			t.Errorf("id %q: want 404, got %d", id, resp.StatusCode)
		}
	}
}
