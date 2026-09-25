package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// The public album pages (task 334).

// fixtureCredit is the photographer's credit the album fixture carries (task 393).
//
// A distinctive name, because the point of it is to be **found** on the rendered page — the opposite of
// `publicprivacy_test.go`'s `leakName`, which exists to be absent. Keeping the two in the same shape makes the
// pair of assertions read as one statement: this surface may name a photographer and may not name a
// participant.
const fixtureCredit = "Foto: Vibeke Krogh"

// albumStore is a stub that holds albums properly rather than returning fixed answers.
//
// Implemented for real — including the publication filter in `BySlug` — because the property under test
// is that an unpublished album is unreachable, and a stub that ignored `published` would make that test
// pass while the behaviour was absent. The same reasoning `stubGlimt.RefsUsedElsewhere` records.
type albumStore struct {
	albums []albumStoreEntry
	err    error
}

type albumStoreEntry struct {
	album     album.Album
	items     []album.Item
	published bool
	deleted   bool
}

func (s *albumStore) Published(string) ([]album.Album, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := []album.Album{}
	for _, e := range s.albums {
		if !e.published || e.deleted {
			continue
		}
		a := e.album
		a.ItemCount = len(e.items)
		if len(e.items) > 0 {
			a.CoverOrdinal = e.items[0].Ordinal
			a.HasCover = true
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *albumStore) BySlug(_, slug string) (album.Album, []album.Item, bool, error) {
	if s.err != nil {
		return album.Album{}, nil, false, s.err
	}
	for _, e := range s.albums {
		if e.album.Slug != slug {
			continue
		}
		// One answer for unpublished, deleted and unknown: the open web must not be able to enumerate
		// drafts.
		if !e.published || e.deleted {
			return album.Album{}, nil, false, nil
		}
		a := e.album
		a.ItemCount = len(e.items)
		if len(e.items) > 0 {
			a.CoverOrdinal = e.items[0].Ordinal
			a.HasCover = true
		}
		return a, e.items, true, nil
	}
	return album.Album{}, nil, false, nil
}

func (s *albumStore) Plottable(string) ([]album.PlottableItem, error) { return nil, s.err }

// albumApp is a public app with two published albums, one draft, and real bytes in the blob store.
func albumApp(t *testing.T) (*application, *albumStore) {
	t.Helper()

	app, _, _ := publicApp(t)
	store := seedAlbums(t, app)
	app.models.Albums = store
	return app, store
}

// seedAlbums builds the album fixture and puts its bytes in the app's blob store.
//
// Real objects rather than invented refs, so the media route can actually serve them, the ETag path is
// exercised, and the removal tests can assert which bytes survived — which is the whole point of task
// 335's sharing check.
func seedAlbums(t *testing.T, app *application) *albumStore {
	t.Helper()

	put := func(body string) string {
		ref, err := app.blobs.Put(context.Background(), []byte(body))
		if err != nil {
			t.Fatalf("seeding the blob store: %v", err)
		}
		return ref.String()
	}

	lat, lng := 55.7332, 12.2648
	return &albumStore{albums: []albumStoreEntry{
		{
			album:     album.Album{ID: "al-1", Slug: "loerdag-morgen", Title: "Lørdag morgen", Description: "Da solen kom", SortOrder: 10},
			published: true,
			items: []album.Item{
				{Ordinal: 0, Ref: put("full-1"), ThumbRef: put("thumb-1"), Caption: "Ved målstregen",
					// A credit line (task 393). The fixture carries one because it is the only field on this
					// surface that may name a person, so every walk over the public pages should meet it.
					Credit: fixtureCredit,
					Width:  1600, Height: 1200, Lat: &lat, Lng: &lng, BoundsVerdict: album.BoundsInside},
				// No caption, and no thumbnail: the page must fall back to the full image.
				{Ordinal: 1, Ref: put("full-2"), Width: 1200, Height: 1600, BoundsVerdict: album.BoundsNone},
			},
		},
		{
			album:     album.Album{ID: "al-2", Slug: "natten", Title: "Natten", SortOrder: 20},
			published: true,
			items: []album.Item{
				{Ordinal: 0, Ref: put("full-3"), ThumbRef: put("thumb-3"), Caption: "Post 4A",
					Width: 1600, Height: 1200, BoundsVerdict: album.BoundsOutside},
			},
		},
		{
			// Deliberately unpublished. Nothing public may show this, which is the point of having it.
			album:     album.Album{ID: "al-draft", Slug: "kladde", Title: "Kladde", SortOrder: 30},
			published: false,
			items: []album.Item{
				{Ordinal: 0, Ref: put("full-draft"), ThumbRef: put("thumb-draft"), Caption: "Hemmelig",
					Width: 1600, Height: 1200},
			},
		},
	}}
}

// **The one name the public surface may carry** (task 393).
//
// The photographer's credit renders, and it renders as its own line under the caption rather than run together
// with it — the caption is what the photograph is *of*, the credit is who took it, and one sentence containing
// both reads as though the photographer were part of the scene.
//
// Paired with `TestNoPublicResponseNamesAPerson`, which asserts a *participant's* name is absent from this same
// page. Together they say the thing precisely: this surface may name somebody who asked to be named, as the
// author of a photograph, and nobody else.
func TestAlbumPageShowsThePhotographersCredit(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	if !strings.Contains(page, fixtureCredit) {
		t.Errorf("the credit line must appear on the album page\n%s", page)
	}
	if !strings.Contains(page, `<span class="credit">`+fixtureCredit+`</span>`) {
		t.Errorf("the credit must be its own element under the caption, not part of the caption's sentence\n%s",
			page)
	}

	// The captionless second item has no credit either, so it must still render no figcaption at all — the
	// credit must not resurrect an empty one.
	if got := strings.Count(page, "<figcaption>"); got != 1 {
		t.Errorf("want exactly one figcaption: one item has a caption and a credit, the other has neither; got %d",
			got)
	}
}

// A photograph with a credit and no caption still gets a figcaption — the credit alone is worth showing.
//
// The template condition is `or .Caption .Credit` rather than keying off the caption, and this is the case that
// distinguishes the two: an uncaptioned photograph by a named photographer must still be attributed.
func TestAlbumPageShowsACreditWithoutACaption(t *testing.T) {
	app, store := albumApp(t)
	// The second item has neither. Give it a credit only.
	store.albums[0].items[1].Credit = "Foto: Jens Balle"
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	if !strings.Contains(page, "Foto: Jens Balle") {
		t.Errorf("a credit must render even with no caption\n%s", page)
	}
	if got := strings.Count(page, "<figcaption>"); got != 2 {
		t.Errorf("want two figcaptions now that both items have something to say, got %d", got)
	}
}

func TestFrontpageListsPublishedAlbums(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026", nil)
	page := string(body)

	for _, want := range []string{
		"Lørdag morgen", "Da solen kom", `href="/2026/album/loerdag-morgen"`,
		"Natten", `href="/2026/album/natten"`,
		// The cover is the album's first item, addressed through the media route.
		"/api/public/albums/al-1/media/0?variant=thumb",
		"2 billeder",
		// **And the singular** (task 387). "Natten" holds one photograph, and this page rendered "1 billeder"
		// to every family that opened it until then — asserting only the plural above was what let that ship.
		"1 billede<",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the frontpage is missing %q\n%s", want, page)
		}
	}

	// Said the other way round too, because "1 billede<" above is also a substring of "1 billeder" without the
	// closing bracket — and a needle that can match the bug it is guarding against is not a guard.
	if strings.Contains(page, "1 billeder") {
		t.Errorf("an album with one photograph must not read \"1 billeder\"\n%s", page)
	}

	// And the empty state must be gone now that there are albums.
	if strings.Contains(page, "Der er ikke lagt billeder op endnu") {
		t.Error("the empty state should not render when albums exist")
	}
}

// **The draft must not leak anywhere.** Not its title, not its slug, not its photographs.
func TestFrontpageHidesUnpublishedAlbums(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026", nil)
	page := string(body)

	for _, forbidden := range []string{"Kladde", "kladde", "al-draft", "Hemmelig"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("an unpublished album must not appear on the frontpage, found %q", forbidden)
		}
	}
}

func TestAlbumPageRendersItsPhotographs(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	page := string(body)

	for _, want := range []string{
		"<h1>Lørdag morgen</h1>", "Da solen kom",
		"/api/public/albums/al-1/media/0?variant=thumb",
		"/api/public/albums/al-1/media/1?variant=thumb",
		"Ved målstregen",
		`loading="lazy"`, `width="1600"`, `height="1200"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the album page is missing %q\n%s", want, page)
		}
	}
}

// The grid must never display a thumbnail larger than the thumbnail actually is (task 398, PRD 023 §7.1).
//
// # Why a test about CSS, in a suite that cannot execute any
//
// Because the number *is* the decision. The stored thumbnail is 320px on its longest edge
// (`glimtThumbEdges`), and this grid used to ask for an 18rem column — a 288px tile, so very nearly the whole
// image, and visibly soft on the 2× display every phone has. An album of 300 photographs was therefore both
// slower and blurrier than it needed to be: we were paying to upscale.
//
// One number fixes it, and the thing most likely to undo it is somebody widening the column again because a
// page "looks too dense" — a change that looks purely cosmetic and is not. This fails when they do, and the
// message says what it costs and what the alternative is.
//
// # The stylesheet is read with its comments stripped
//
// Deliberately, because the comment above the rule quotes `minmax(18rem, 1fr)` in order to explain what was
// wrong with it. A needle that matches the prose explaining the rule is the recurring failure of
// source-reading guards in this repo — four times so far; see `foldBody` in
// `nathejk/table/album/membershipsafety_test.go`.
func TestTheAlbumGridDoesNotUpscaleItsThumbnails(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	// Every thumbnail sits in the square frame. It is what makes a row of tiles line up, and what holds a
	// tile's space before its bytes arrive — on the img itself the ratio collapses while the image is
	// loading, and the grid jumps as each photograph lands.
	if got, want := strings.Count(page, `<span class="frame">`), 2; got != want {
		t.Errorf("want one square frame per item (%d), got %d\n%s", want, got, page)
	}

	css := stripCSSComments(page)
	i := strings.Index(css, ".photos {")
	if i < 0 {
		t.Fatalf("no .photos rule in the page's stylesheet\n%s", css)
	}
	rule := css[i:]
	if j := strings.Index(rule, "}"); j >= 0 {
		rule = rule[:j]
	}

	// 10rem is 160px at a 16px root, which a 320px thumbnail covers exactly at 2×.
	const maxTileRem = 10.0

	m := regexp.MustCompile(`minmax\(([0-9.]+)rem`).FindStringSubmatch(rule)
	if m == nil {
		t.Fatalf("the .photos grid no longer sizes its columns with minmax(…rem), so this guard cannot "+
			"check them: %q", rule)
	}
	rem, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unreadable column width %q: %v", m[1], err)
	}
	if rem > maxTileRem {
		t.Errorf("the album grid asks for a %grem column, i.e. a %.0fpx tile, while the stored thumbnail is "+
			"%dpx on its longest edge — so every photograph on the page is upscaled, and soft on any 2\u00d7 "+
			"display. Keep the column at or below %grem, or serve a larger rendition (PRD 023 §7.9).",
			rem, rem*16, glimtThumbEdges[0], maxTileRem)
	}
}

// stripCSSComments removes every /* … */ from a rendered page.
//
// So that a guard about a CSS rule cannot be satisfied — or defeated — by the comment that explains the rule.
// The public site's stylesheet is inline and heavily commented, and those comments quote the declarations they
// argue against, which is exactly the text a naive `strings.Contains` would match.
func stripCSSComments(page string) string {
	var out strings.Builder
	for {
		i := strings.Index(page, "/*")
		if i < 0 {
			out.WriteString(page)
			return out.String()
		}
		out.WriteString(page[:i])
		rest := page[i+2:]
		j := strings.Index(rest, "*/")
		if j < 0 {
			// An unterminated comment: everything after it is comment, so there is nothing more to keep.
			return out.String()
		}
		page = rest[j+2:]
	}
}

// Every item gets its own tag, and every one of them is reachable without script.
//
// That is the no-carousel decision (PRD 019 task 323) restated for albums: a desktop visitor must reach every
// photograph without a swipe.
//
// # This test used to forbid the string "<script" and no longer can
//
// Task 403 gave the page a viewer, so it now loads one deferred, same-origin script. The assertion it replaces
// was a proxy for what actually matters, and keeping the proxy would have meant either deleting the test or
// leaving the page without the enhancement. So the requirement is written out instead, in three parts, all of
// which were true before the viewer and must stay true after it:
//
//   - **every photograph is reachable by a link**, which is what a visitor without the viewer uses;
//   - **no inline script**, so the page carries no behaviour of its own and nothing that needs a nonce;
//   - **every script is deferred and same-origin**, so nothing blocks the page and no third party learns who
//     looked at which photograph.
func TestAlbumPageRendersEveryItemWithoutACarousel(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	if got := strings.Count(page, "<img "); got != 2 {
		t.Errorf("want one img per item (2), got %d\n%s", got, page)
	}
	// One link per photograph, to the photograph. This is the fallback the whole progressive-enhancement
	// argument rests on (PRD 023 §8), so it is asserted rather than assumed.
	if got := strings.Count(page, `<a class="tile" href="/api/public/albums/al-1/media/`); got != 2 {
		t.Errorf("want one link per photograph (2), got %d\n%s", got, page)
	}
	lower := strings.ToLower(page)
	for _, forbidden := range []string{"onclick=", "scroll-snap"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("the album page must work without script, found %q", forbidden)
		}
	}
	assertScriptsAreDeferredAndOurs(t, page)
}

// assertScriptsAreDeferredAndOurs holds the three rules a public page's script tags must obey.
//
// Shared by the album-page tests because the rules are about the page rather than about one behaviour, and
// because the failure they guard against — a CDN tag, or an inline block — arrives in a change that is about
// something else entirely.
func assertScriptsAreDeferredAndOurs(t *testing.T, page string) {
	t.Helper()

	for _, tag := range regexp.MustCompile(`<script[^>]*>`).FindAllString(page, -1) {
		if !strings.Contains(tag, " src=") {
			t.Errorf("inline script on a public page: %s", tag)
			continue
		}
		if !strings.Contains(tag, " defer") {
			t.Errorf("script is not deferred, so it blocks the page: %s", tag)
		}
		if !strings.Contains(tag, `src="/`) {
			t.Errorf("script is not same-origin, so a third party could log who looked at which photograph: %s",
				tag)
		}
	}
}

// The album page hands the viewer its whole input, and declares what it may do (task 403, PRD 023 §7.4, §7.7).
//
// # Why the declaration is asserted, not just the attributes
//
// `data-viewer-actions="share,fullscreen"` is the mechanism by which the public viewer has **no editing
// controls**. Not a hidden button, not a disabled one: the viewer builds only what the page names, so there is
// nothing on a public page for anybody to un-hide. A future edit that added `caption` here would be a public
// caption editor, and since no test can execute the viewer's JavaScript, this line of markup is where that gets
// caught.
func TestTheAlbumPageWiresTheViewer(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	for _, want := range []struct{ needle, why string }{
		{`data-viewer-actions="share,fullscreen"`,
			"share and fullscreen, and nothing that edits: the public viewer's controls are what this page names"},
		{`data-viewer-history="foto"`,
			"the current photograph is reflected in ?foto=, which is the parameter task 401 taught the server"},
		{`data-share-title="Lørdag morgen — Nathejk 2026"`,
			"the share sheet's title is the album's, never a caption: a caption is free text and the one place a " +
				"person's name could plausibly land"},
		{`data-viewer-item`, "each tile announces itself to the viewer"},
		{`data-viewer-ordinal="0"`, "and carries the ordinal the deep link names"},
		{`data-full="/api/public/albums/al-1/media/0"`, "the display image, for the overlay"},
		{`data-thumb="/api/public/albums/al-1/media/0?variant=thumb"`, "the thumbnail, for the filmstrip"},
		{`data-caption="Ved målstregen"`, "the caption reaches the viewer without a request"},
		{`data-credit="` + fixtureCredit + `"`, "and so does the credit"},
		{`id="foto-0"`, "the anchor half of the deep link"},
	} {
		if !strings.Contains(page, want.needle) {
			t.Errorf("the album page is missing %s — %s", want.needle, want.why)
		}
	}

	// The viewer must not be able to edit anything from here, and the way to be sure is that the page never asks
	// for a control that could. Read the declaration back out and check every name in it, rather than hunting for
	// substrings: an allowlist cannot be defeated by a spelling nobody thought of.
	for _, declared := range regexp.MustCompile(`data-viewer-actions="([^"]*)"`).FindAllStringSubmatch(page, -1) {
		for _, name := range strings.Split(declared[1], ",") {
			switch strings.TrimSpace(name) {
			case "share", "fullscreen":
			default:
				t.Errorf("the public page declares the viewer action %q; only share and fullscreen belong on an "+
					"unauthenticated surface", name)
			}
		}
	}
	if strings.Contains(page, "data-caption-endpoint") {
		t.Error("the public page must not carry a write endpoint")
	}

	// And it is an enhancement: both assets are ours, deferred, and hashed so a fix is not stuck in a cache.
	if !strings.Contains(page, `<script src="`+viewerAssetPath("viewer.js")+`" defer></script>`) {
		t.Errorf("want the viewer's script, deferred and versioned\n%s", page)
	}
	if !strings.Contains(page, `<link rel="stylesheet" href="`+viewerAssetPath("viewer.css")+`">`) {
		t.Errorf("want the viewer's stylesheet\n%s", page)
	}
}

// The caption's visible line went; **the credit's did not** (task 403, PRD 023 §7.1).
//
// PRD 023 traded the caption under the tile for the viewer's info panel: at 150px there is no room, and the
// caption is still in the alt text and in `data-caption`. The credit is a different kind of thing. It is a
// published attribution — a photographer asked to be named, and PRD 011's "names no person" claim was formally
// narrowed to allow exactly this (task 393). An attribution that only renders once a script has run is an
// attribution we stop making for everybody whose script did not run, and no layout change is entitled to decide
// that.
func TestTheCreditStaysOnThePageWhileTheCaptionMovesToTheViewer(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	// Visible, in its own element, with no script involved.
	if !strings.Contains(page, `<figcaption><span class="credit">`+fixtureCredit+`</span></figcaption>`) {
		t.Errorf("the photographer's credit must still be readable without the viewer\n%s", page)
	}
	// The caption is present as data and as alt text, and no longer as a line under the tile.
	if !strings.Contains(page, `data-caption="Ved målstregen"`) ||
		!strings.Contains(page, `alt="Ved målstregen"`) {
		t.Errorf("the caption must stay in the markup for the viewer and for a screen reader\n%s", page)
	}
	if strings.Contains(page, `<figcaption>Ved målstregen`) {
		t.Error("the caption's visible line under the tile should have moved into the viewer's info panel")
	}
}

// An item with no caption must not reserve space for one.
func TestAlbumPageOmitsAnAbsentCaption(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	if got := strings.Count(page, "<figcaption>"); got != 1 {
		t.Errorf("want exactly one figcaption (only one item has a caption), got %d", got)
	}
	// The captionless item still needs alt text, falling back to the album's title.
	if !strings.Contains(page, `alt="Billede fra Lørdag morgen"`) {
		t.Errorf("want fallback alt text for the captionless item\n%s", page)
	}
}

// **No coordinate may reach an album page.** Positions reach the public only through the map endpoint,
// which is where that decision is made and tested — not through a template nobody reviewed for it.
func TestAlbumPageCarriesNoCoordinates(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/2026", "/2026/album/loerdag-morgen"} {
		_, body := getPublic(t, srv.URL+path, nil)
		page := string(body)
		for _, forbidden := range []string{"55.7332", "12.2648", "latitude", "longitude",
			album.BoundsInside, album.BoundsOutside} {
			if strings.Contains(page, forbidden) {
				t.Errorf("%s leaks %q: positions belong to the map endpoint alone", path, forbidden)
			}
		}
	}
}

// Unknown, unpublished and deleted must be one answer, or the open web can enumerate drafts.
func TestAlbumPageAnswers404IdenticallyForDraftsAndUnknowns(t *testing.T) {
	app, store := albumApp(t)
	store.albums[1].deleted = true // "natten"
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	var first string
	for i, slug := range []string{"kladde", "natten", "findes-ikke"} {
		resp, body := getPublic(t, srv.URL+"/2026/album/"+slug, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: want 404, got %d", slug, resp.StatusCode)
		}
		if i == 0 {
			first = string(body)
		} else if string(body) != first {
			t.Errorf("%s: the refusal must be identical for drafts, deletions and unknowns", slug)
		}
	}
}

func TestAlbumPageIsNotIndexedAndShortCached(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	if got := resp.Header.Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
		t.Errorf("want noindex, got %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "max-age=60") {
		t.Errorf("want the short window task 335 depends on, got %q", got)
	}
}

// bigAlbum replaces an album's items with n synthetic ones, for the paging tests (task 399).
//
// The captions are numbered so a test can say *which* window it is looking at rather than only how big it is.
// Counting tiles cannot distinguish page 2 from a second copy of page 1, and that is exactly the bug an
// off-by-one in the offset produces.
func bigAlbum(t *testing.T, store *albumStore, slug string, n int) {
	t.Helper()

	for i := range store.albums {
		if store.albums[i].album.Slug != slug {
			continue
		}
		items := make([]album.Item, 0, n)
		for ordinal := 0; ordinal < n; ordinal++ {
			items = append(items, album.Item{
				Ordinal: ordinal,
				Caption: fmt.Sprintf("Billede nr %d", ordinal),
				Width:   1600, Height: 1200,
				BoundsVerdict: album.BoundsNone,
			})
		}
		store.albums[i].items = items
		return
	}
	t.Fatalf("no album with slug %q to enlarge", slug)
}

// The window arithmetic (task 399), table-driven and away from HTTP.
//
// # Why this is not only tested through the page
//
// Because the failure it guards against is an off-by-one, and an off-by-one reaches the page as "the link is
// missing" or "one photograph is on both pages" — symptoms that are a long way from the line that caused them.
// The end-to-end test below asserts the page; this asserts the arithmetic, including the cases nobody would
// think to click.
func TestTheAlbumWindowCapsAndClamps(t *testing.T) {
	items := func(n int) []album.Item {
		out := make([]album.Item, n)
		for i := range out {
			out[i] = album.Item{Ordinal: i}
		}
		return out
	}

	for _, tc := range []struct {
		name      string
		count     int
		side      string
		wantFirst int
		wantLen   int
		wantSide  int
		wantMore  bool
	}{
		// The ordinary album PRD 011 described: one page, no control to press, and `side` is not even consulted.
		{"a short album is one page", 40, "", 0, 40, 1, false},
		{"a short album ignores side", 40, "7", 0, 40, 1, false},
		{"exactly the cap still has no next page", albumPageCap, "", 0, albumPageCap, 1, false},
		{"one over the cap splits", albumPageCap + 1, "", 0, albumPageCap, 1, true},
		{"the second page is the remainder", albumPageCap + 1, "2", albumPageCap, 1, 2, false},
		{"a middle page has a next", albumPageCap*2 + 5, "2", albumPageCap, albumPageCap, 2, true},
		{"the last page has none", albumPageCap*2 + 5, "3", albumPageCap * 2, 5, 3, false},
		// Wrong input is page one or the last page — never an error and never an empty grid.
		{"a side past the end clamps to the last", albumPageCap*2 + 5, "99", albumPageCap * 2, 5, 3, false},
		{"nonsense is page one", albumPageCap + 1, "abc", 0, albumPageCap, 1, true},
		{"zero is page one", albumPageCap + 1, "0", 0, albumPageCap, 1, true},
		{"negative is page one", albumPageCap + 1, "-3", 0, albumPageCap, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, side, more := albumPageWindow(items(tc.count), tc.side)
			if len(got) != tc.wantLen {
				t.Errorf("got %d items, want %d", len(got), tc.wantLen)
			}
			if len(got) > 0 && got[0].Ordinal != tc.wantFirst {
				t.Errorf("window starts at ordinal %d, want %d", got[0].Ordinal, tc.wantFirst)
			}
			if side != tc.wantSide {
				t.Errorf("resolved side %d, want %d", side, tc.wantSide)
			}
			if more != tc.wantMore {
				t.Errorf("hasMore = %v, want %v", more, tc.wantMore)
			}
			if len(got) > albumPageCap {
				t.Errorf("a response carries %d items, which is past the cap of %d", len(got), albumPageCap)
			}
		})
	}
}

// A big album is capped, and the rest is reachable by a **link** rather than by script (task 399).
//
// The no-script half is the point: PRD 011 §8 requires this page to work with JavaScript disabled, and after
// PRD 023 the album page is the one public page that will grow a JavaScript viewer. If the way to photograph
// 201 is ever an event listener, this test is what should stop it.
func TestABigAlbumIsCappedWithAPlainLink(t *testing.T) {
	app, store := albumApp(t)
	bigAlbum(t, store, "loerdag-morgen", albumPageCap+50)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	first := string(body)

	if got := strings.Count(first, `<span class="frame">`); got != albumPageCap {
		t.Errorf("the first page carries %d photographs, want the cap of %d", got, albumPageCap)
	}
	if !strings.Contains(first, `href="/2026/album/loerdag-morgen?side=2"`) {
		t.Errorf("want a real link to the next page\n%s", first)
	}
	if !strings.Contains(first, "Vis flere") {
		t.Error("the link needs its Danish label")
	}
	// The window really is the first one, and the tail really is absent — counting tiles alone cannot tell a
	// correct first page from one that rendered the wrong 200.
	if !strings.Contains(first, `data-caption="Billede nr 0"`) ||
		strings.Contains(first, `data-caption="Billede nr 200"`) {
		t.Errorf("the first page should hold ordinals 0–199\n%s", first)
	}
	// Reaching the rest must not need script. The link above is the whole mechanism; this checks nothing has
	// quietly replaced it with a handler — and that the viewer's own script stays a deferred, same-origin
	// enhancement rather than a dependency (task 403).
	if strings.Contains(strings.ToLower(first), "hx-get") {
		t.Error("paging must not become an htmx swap on a page that has to work without script")
	}
	assertScriptsAreDeferredAndOurs(t, first)

	_, body = getPublic(t, srv.URL+"/2026/album/loerdag-morgen?side=2", nil)
	second := string(body)

	if got := strings.Count(second, `<span class="frame">`); got != 50 {
		t.Errorf("the second page carries %d photographs, want the remaining 50", got)
	}
	if !strings.Contains(second, `data-caption="Billede nr 200"`) ||
		strings.Contains(second, `data-caption="Billede nr 199"`) {
		t.Errorf("the second page should hold ordinals 200–249\n%s", second)
	}
	// And no link onward from the last page: a control promising a page that does not exist is the failure the
	// admin sheet's "Hent flere" was built to avoid, and it is worse here because a visitor cannot tell.
	if strings.Contains(second, "?side=3") {
		t.Errorf("the last page must not offer a next one\n%s", second)
	}
}

// The deep link lands on the page holding the photograph (task 401).
//
// # The case that matters is the one past the cap
//
// Within the first page every implementation works, including the wrong ones, which is why this walks the cap
// boundary. A shared link is opened days later on somebody else's device (PRD 023 §2) — there is no app state
// to fall back on and no second chance to get it right.
func TestTheFotoDeepLinkRendersThePageHoldingIt(t *testing.T) {
	app, store := albumApp(t)
	bigAlbum(t, store, "loerdag-morgen", albumPageCap+50)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, tc := range []struct {
		query      string
		want, deny string
		why        string
	}{
		{"?foto=0", "Billede nr 0", "Billede nr 200",
			"ordinal 0 is a real ordinal and must not be treated as a missing parameter"},
		{"?foto=210", "Billede nr 210", "Billede nr 0",
			"an ordinal past the cap must land on its own page, not on the first one"},
		{"?foto=249", "Billede nr 249", "Billede nr 199",
			"the last photograph is reachable by link"},
		// `foto` wins over `side`: the photograph is what the sender meant, and the window is only how this page
		// happens to be cut up today.
		{"?side=1&foto=210", "Billede nr 210", "Billede nr 0",
			"a stale side beside a foto must not win"},
		{"?side=2&foto=3", "Billede nr 3", "Billede nr 210",
			"and that is true in both directions"},
		// Rubbish and rot land on the album, never on an error.
		{"?foto=abc", "Billede nr 0", "Billede nr 210", "nonsense falls back to the first page"},
		{"?foto=-4", "Billede nr 0", "Billede nr 210", "a negative ordinal falls back"},
		{"?foto=99999", "Billede nr 0", "Billede nr 210",
			"an ordinal that no longer exists — the photograph was taken down — lands on the album"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			resp, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen"+tc.query, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("want 200 — a half-rotted link must land on the album, not on an error — got %d",
					resp.StatusCode)
			}
			page := string(body)
			if !strings.Contains(page, `data-caption="`+tc.want+`"`) {
				t.Errorf("want %q on the page: %s", tc.want, tc.why)
			}
			if strings.Contains(page, `data-caption="`+tc.deny+`"`) {
				t.Errorf("did not want %q on the page: %s", tc.deny, tc.why)
			}
			// The anchor is the half of the link the browser uses. Without it the server lands on the right page
			// and the visitor still has to hunt for the photograph.
			if !strings.Contains(page, `<figure id="foto-`) {
				t.Errorf("every tile needs its anchor\n%s", page)
			}
		})
	}
}

// An ordinal is not an index, and a division would have shipped that bug (task 401).
//
// `album_item` rows are soft-deleted and a photograph deleted from the library stops satisfying `BySlug`'s
// join — which is how the projection intends a deletion to take effect everywhere at once. So a long-lived
// album hands back sparse ordinals, and `ordinal / albumPageCap + 1` would send a visitor to a page the
// photograph is not on.
//
// # The fixture has to be past the cap, and the first version of this test was not
//
// Written first with three sparsely-numbered photographs, it passed with the division in place — because
// `albumPageWindow` returns early for an album at or under the cap and never consults the side at all. A guard
// that cannot fail is worse than no guard, so the fixture is now 250 items with **one** deliberately high
// ordinal near the front: the two implementations disagree about which page holds it, and only one of them is
// right.
func TestTheFotoDeepLinkCountsPositionsRatherThanOrdinals(t *testing.T) {
	app, store := albumApp(t)
	bigAlbum(t, store, "loerdag-morgen", albumPageCap+50)

	// Position 5 — firmly on page one — carrying an ordinal from a much larger album that has since been
	// pruned. Dividing it by the cap lands on page 21, which clamps to the last page, which is nowhere near it.
	const strandedOrdinal = 4000
	for i := range store.albums {
		if store.albums[i].album.Slug == "loerdag-morgen" {
			store.albums[i].items[5].Ordinal = strandedOrdinal
		}
	}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen?foto=4000", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	page := string(body)

	if !strings.Contains(page, `data-caption="Billede nr 5"`) {
		t.Errorf("the deep link must land on the page the item is *positioned* on, not on ordinal/cap\n%s", page)
	}
	if strings.Contains(page, `data-caption="Billede nr 200"`) {
		t.Error("a high ordinal near the front of an album is not a high position: this is page two, which " +
			"means the side was derived by dividing the ordinal")
	}
	if !strings.Contains(page, `id="foto-4000"`) {
		t.Error("the anchor must carry the item's real ordinal, since that is what the link names")
	}
}

// An ordinary album gets no paging furniture at all.
//
// Worth its own test because the cap is insurance, not a feature: PRD 011's albums are three to five dozen
// photographs, and a "Vis flere" link under a two-screen album would be a control that does nothing useful and
// a page that looks truncated when it is complete.
func TestAnOrdinaryAlbumHasNoPagingControl(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen?side=4", nil)
	page := string(body)

	if strings.Contains(page, "Vis flere") || strings.Contains(page, "?side=") {
		t.Errorf("a two-photograph album needs no paging control\n%s", page)
	}
	// And the nonsense `side` did not empty it: the address names a real album, so it renders one.
	if !strings.Contains(page, "Ved målstregen") {
		t.Errorf("an out-of-range side must still render the album\n%s", page)
	}
}

func TestAlbumMediaServesTheThumbnailAndFallsBack(t *testing.T) {
	app, store := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// Item 0 has a thumbnail.
	resp, body := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0?variant=thumb", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if string(body) != "thumb-1" {
		t.Errorf("want the thumbnail bytes, got %q", body)
	}

	// Item 1 has none, so asking for a thumbnail must fall back to the full image rather than 404.
	// Losing a thumbnail should cost bandwidth, not the photograph.
	resp, body = getPublic(t, srv.URL+"/api/public/albums/al-1/media/1?variant=thumb", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 from the fallback, got %d", resp.StatusCode)
	}
	if string(body) != "full-2" {
		t.Errorf("want the full image as fallback, got %q", body)
	}

	// Without the variant, the full image.
	_, body = getPublic(t, srv.URL+"/api/public/albums/al-1/media/0", nil)
	if string(body) != "full-1" {
		t.Errorf("want the full image, got %q", body)
	}

	_ = store
}

// **The media route's own visibility check.** Bytes are addressed by id and ordinal, so without it an
// id would be enough to pull a draft album's photograph off an unauthenticated route.
func TestAlbumMediaRefusesADraftAlbum(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, variant := range []string{"", "?variant=thumb"} {
		resp, _ := getPublic(t, srv.URL+"/api/public/albums/al-draft/media/0"+variant, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("a draft album's media must answer 404, got %d", resp.StatusCode)
		}
	}
}

func TestAlbumMediaRefusesRubbish(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{
		"/api/public/albums/al-1/media/99", // no such ordinal
		"/api/public/albums/al-1/media/x",  // not a number
		"/api/public/albums/al-1/media/-1", // negative
		"/api/public/albums/findes-ikke/media/0",
	} {
		resp, _ := getPublic(t, srv.URL+path, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d", path, resp.StatusCode)
		}
	}
}

// Public and immutable, which is safe precisely because the answer does not depend on who asked.
func TestAlbumMediaIsPubliclyCacheable(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0?variant=thumb", nil)
	cache := resp.Header.Get("Cache-Control")
	if !strings.Contains(cache, "public") || !strings.Contains(cache, "immutable") {
		t.Errorf("want a public immutable cache header, got %q", cache)
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("want an ETag so a revisit costs a 304 rather than a transfer")
	}
}

// The album pages must ignore the session like the rest of the surface.
func TestAlbumPagesIgnoreTheSession(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for _, path := range []string{"/2026/album/loerdag-morgen", "/2026/album/kladde"} {
		_, anonymous := getPublic(t, srv.URL+path, nil)
		_, signedIn := getPublic(t, srv.URL+path, cookies)
		if string(anonymous) != string(signedIn) {
			t.Errorf("%s differs for a signed-in member", path)
		}
	}
}

// A failing album read must not take the frontpage down: the lookup and the glimt strip are unaffected.
func TestFrontpageSurvivesAFailingAlbumRead(t *testing.T) {
	app, store := albumApp(t)
	store.err = errors.New("database is down")
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/2026", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 with a degraded section, got %d", resp.StatusCode)
	}
	page := string(body)
	if !strings.Contains(page, "Der er ikke lagt billeder op endnu") {
		t.Errorf("want the albums section to degrade to its empty state\n%s", page)
	}
	if !strings.Contains(page, "Find din patrulje") {
		t.Error("the rest of the page must still render")
	}
}

// With no album projection the section is empty and the page is fine. 503 is for the album *page*,
// where a 404 would tell a curator their album had vanished.
func TestAlbumPageIsUnavailableRatherThanMissingWithoutAProjection(t *testing.T) {
	app, _ := albumApp(t)
	app.models.Albums = nil
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", resp.StatusCode)
	}

	resp, body := getPublic(t, srv.URL+"/2026", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("the frontpage must still render, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Der er ikke lagt billeder op endnu") {
		t.Error("want the albums empty state")
	}
}

// Albums appear in the curator's order, not creation order: "3-5 albums" is an editorial sequence.
func TestFrontpageAlbumsFollowCuratorOrder(t *testing.T) {
	app, store := albumApp(t)
	// Reverse the sort orders; the stub returns them in slice order, so this asserts the *reader*
	// respects what the projection hands back rather than re-sorting.
	store.albums[0].album.SortOrder = 20
	store.albums[1].album.SortOrder = 10
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026", nil)
	page := string(body)

	first := strings.Index(page, "loerdag-morgen")
	second := strings.Index(page, "natten")
	if first < 0 || second < 0 {
		t.Fatal("both albums should be listed")
	}
	// The stub preserves slice order, which is the projection's contract (ORDER BY sortOrder), so the
	// page must not impose an order of its own.
	if first > second {
		t.Error("the page must render albums in the order the projection returned them")
	}
}
