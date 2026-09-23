package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the frontpage is missing %q\n%s", want, page)
		}
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

// Every item gets its own tag. That is the no-carousel decision (PRD 019 task 323) restated for albums:
// a desktop visitor must reach every photograph without a swipe and without script.
func TestAlbumPageRendersEveryItemWithoutACarousel(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	if got := strings.Count(page, "<img "); got != 2 {
		t.Errorf("want one img per item (2), got %d\n%s", got, page)
	}
	lower := strings.ToLower(page)
	for _, forbidden := range []string{"<script", "onclick=", "scroll-snap"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("the album page must work without script, found %q", forbidden)
		}
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
