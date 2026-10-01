package main

import (
	"bytes"
	"context"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/album"
)

// The 800px rendition end to end (task 409, PRD 023 §7.9).
//
// The number itself is the decision: a 390pt phone shows about 800px at 2×, so serving it the 1600px display
// image is four times the pixels for no visible gain — times however many photographs somebody swipes
// through. These tests are about the three ways that saving can be silently lost: the rendition not being
// produced, the route not serving it, and the tile not offering it.

// mediumAlbumApp seeds one album item that has all three renditions, with real decodable bytes.
func mediumAlbumApp(t *testing.T) (*application, *album.Item, *httptest.Server) {
	t.Helper()

	app, store := albumApp(t)

	put := func(data []byte) string {
		ref, err := app.blobs.Put(context.Background(), data)
		if err != nil {
			t.Fatalf("seeding: %v", err)
		}
		return ref.String()
	}
	item := album.Item{
		Ordinal:       0,
		Ref:           put(testImage(t, 1600, 1200)),
		MediumRef:     put(testImage(t, 800, 600)),
		ThumbRef:      put(testImage(t, 320, 240)),
		Width:         1600,
		Height:        1200,
		BoundsVerdict: album.BoundsNone,
	}
	store.albums[0].items = []album.Item{item}

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return app, &item, srv
}

// decodedWidth fetches a variant and reports the width of the JPEG it got.
//
// The width is the assertion rather than the ref, because the ref only proves which row was read while the
// width proves which *bytes* went over the wire — and it is the bytes a phone pays for.
func decodedWidth(t *testing.T, srv *httptest.Server, url string) int {
	t.Helper()

	resp, body := getPublic(t, srv.URL+url, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d", url, resp.StatusCode)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("GET %s did not return a decodable JPEG: %v", url, err)
	}
	return cfg.Width
}

// The point of the whole task: three distinct sizes on one URL, chosen by one query parameter.
func TestTheAlbumMediaRouteServesThreeSizes(t *testing.T) {
	_, _, srv := mediumAlbumApp(t)

	for _, c := range []struct {
		variant string
		want    int
	}{
		{"", 1600},
		{"?variant=medium", 800},
		{"?variant=thumb", 320},
	} {
		if got := decodedWidth(t, srv, "/api/public/albums/al-1/media/0"+c.variant); got != c.want {
			t.Errorf("variant %q served a %dpx image, want %dpx", c.variant, got, c.want)
		}
	}
}

// No backfill, and this is what makes that safe (task 409's §4/§7.9 decision).
//
// A photograph uploaded before the rendition existed has `mediumRef = ""`. Asking for `variant=medium` must
// answer with the display image rather than 404 — correct rather than degraded, and the reason no existing
// row had to be touched.
func TestAMediumVariantWithNoRenditionFallsBackToTheDisplayImage(t *testing.T) {
	app, store := albumApp(t)

	full, err := app.blobs.Put(context.Background(), testImage(t, 1600, 1200))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	// Exactly the pre-409 shape: a full rendition, no medium.
	store.albums[0].items = []album.Item{{
		Ordinal: 0, Ref: full.String(), Width: 1600, Height: 1200, BoundsVerdict: album.BoundsNone,
	}}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0?variant=medium", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("a photograph with no medium rendition must still serve, got %d", resp.StatusCode)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("not a decodable JPEG: %v", err)
	}
	if cfg.Width != 1600 {
		t.Errorf("fell back to a %dpx image, want the 1600px display image", cfg.Width)
	}
	// And the ETag names what was actually sent, so no cache is told the display image is the 800px one.
	if got := resp.Header.Get("ETag"); !strings.Contains(got, full.String()) {
		t.Errorf("ETag = %q, want the full rendition's ref %q", got, full)
	}
}

// The frontpage's album covers (task 461).
//
// # The arithmetic this is about
//
// A cover card is 12rem at its narrowest and the **full column width on a phone** — about 330px, which is 660
// device pixels at 2x. The thumbnail is 320px on its *longest* edge and the crop is square, so a landscape
// photograph brings 240px of vertical detail to a box asking for 660: a 2.7x upscale, reported as "somewhat
// blurry". The 800px rendition already existed; the frontpage simply never offered it.
//
// Note what is deliberately **not** changed: the album page's own grid stays on the thumbnail. Its tiles are
// 10rem, where the same file is very nearly a genuine 2x image, and offering the medium there would have an album
// of 300 photographs fetch and decode 300 of them (task 398 sized that grid on purpose).

// coverTag returns the frontpage's cover <img> for one album, so an assertion is about that tag and not about
// some other image on the page.
func coverTag(t *testing.T, page, albumID string) string {
	t.Helper()
	at := strings.Index(page, `<img src="/api/public/albums/`+albumID+`/media/`)
	if at < 0 {
		t.Fatalf("no cover image for %s on the frontpage\n%s", albumID, page)
	}
	end := strings.Index(page[at:], ">")
	if end < 0 {
		t.Fatalf("unterminated cover tag for %s", albumID)
	}
	return page[at : at+end+1]
}

func TestTheFrontpageCoverOffersTheMediumRendition(t *testing.T) {
	_, item, srv := mediumAlbumApp(t)

	_, body := getPublic(t, srv.URL+"/2026", nil)
	tag := coverTag(t, string(body), "al-1")

	base := "/api/public/albums/al-1/media/" + item.Ref
	for _, want := range []struct{ needle, why string }{
		{`src="` + base + `?variant=thumb"`,
			"the thumbnail stays the src: it is what a browser ignoring srcset gets, and what a dense desktop " +
				"row wants"},
		{base + "?variant=thumb 320w", "the small candidate, at its real width"},
		{base + "?variant=medium 800w", "and the one that fixes the phone"},
		{`sizes=`, "without sizes the browser assumes 100vw and always takes the larger file"},
	} {
		if !strings.Contains(tag, want.needle) {
			t.Errorf("the cover tag is missing %q: %s\n%s", want.needle, want.why, tag)
		}
	}

	// **Addressed by ref, and only the display ref.** The derived renditions are reached with `?variant=`, so
	// their own hashes never leave the server — the bound the invariant was narrowed within (PRD 022 §8.4).
	for name, ref := range map[string]string{"thumb": item.ThumbRef, "medium": item.MediumRef} {
		if strings.Contains(tag, ref) {
			t.Errorf("the %s rendition's own ref must not appear in the cover tag\n%s", name, tag)
		}
	}
}

// No medium rendition, no srcset — **not** a srcset naming a URL the server would answer by falling back.
//
// That fallback is correct at the byte level and a lie at the label level: 1600px bytes behind an 800w
// descriptor. A browser does arithmetic with those numbers, so it would pick the "smaller" file and get the
// largest one on the narrowest screen, which is the opposite of the point. The default album fixture has no
// medium on its first item, which is every photograph uploaded before task 409.
func TestTheFrontpageCoverOffersNoSrcsetWithoutAMedium(t *testing.T) {
	app, store := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	if store.albums[0].items[0].MediumRef != "" {
		t.Fatal("this test needs a cover with no medium rendition")
	}

	tag := coverTag(t, string(secondOf(getPublic(t, srv.URL+"/2026", nil))), "al-1")
	if strings.Contains(tag, "srcset") {
		t.Errorf("a cover with no 800px rendition must offer one candidate and no srcset\n%s", tag)
	}
	// It still renders, at the size it has. Losing a rendition costs sharpness, never the photograph.
	if !strings.Contains(tag, "?variant=thumb") {
		t.Errorf("the cover must still render from the thumbnail\n%s", tag)
	}
}

// And the cover's bytes are cacheable for a year again (task 461).
//
// This is the regression the ref fixes rather than a new property. Task 456 had to stop serving **ordinal** URLs
// `immutable`, because PRD 024 made an ordinal's meaning move under a re-sort — and the frontpage addressed its
// covers by ordinal. That quietly turned the one page an entire event opens on the Sunday morning into a
// 60-second cache for its largest assets.
func TestTheFrontpageCoverIsCacheableForAYear(t *testing.T) {
	_, item, srv := mediumAlbumApp(t)

	for _, variant := range []string{"?variant=thumb", "?variant=medium"} {
		resp, _ := getPublic(t, srv.URL+"/api/public/albums/al-1/media/"+item.Ref+variant, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: want 200, got %d", variant, resp.StatusCode)
		}
		if cache := resp.Header.Get("Cache-Control"); !strings.Contains(cache, "immutable") {
			t.Errorf("%s: the cover is content-addressed, so it must be immutable; got %q", variant, cache)
		}
	}
}

// secondOf drops a getPublic response and keeps its body, for the one-liners above.
func secondOf(_ *http.Response, body []byte) []byte { return body }

// The tile offers `data-medium` **only** when the photograph has one.
//
// An attribute that was always present would name a URL the server answers by falling back — correct bytes
// under an `800w` label that is a lie, which is the one thing a `srcset` candidate must not be. The viewer
// then emits no `srcset` at all for such a photograph (task 410).
func TestTheAlbumTileOffersTheMediumRenditionOnlyWhenItExists(t *testing.T) {
	_, item, srv := mediumAlbumApp(t)

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)
	// The **display** ref plus `?variant=medium`: one address, three renditions. That is why the 800px
	// rendition's own hash never appears in the page — see TestTheAlbumPageNeverPutsADerivedRenditionRefInItsHTML.
	if !strings.Contains(page, `data-medium="/api/public/albums/al-1/media/`+item.Ref+`?variant=medium"`) {
		t.Errorf("the tile does not offer the medium rendition, so task 410's srcset has one candidate\n%s", page)
	}
}

func TestTheAlbumTileOmitsDataMediumWithoutARendition(t *testing.T) {
	app, store := albumApp(t)
	full, err := app.blobs.Put(context.Background(), testImage(t, 1600, 1200))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	store.albums[0].items = []album.Item{{
		Ordinal: 0, Ref: full.String(), Width: 1600, Height: 1200, BoundsVerdict: album.BoundsNone,
	}}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	if strings.Contains(string(body), "data-medium") {
		t.Errorf("a photograph with no medium rendition must not advertise one: the URL would resolve to the "+
			"display image under an 800w label\n%s", body)
	}
}

// **No derived rendition's hash may appear in a public payload** — narrowed on 2026-09-28, and the narrowing is
// the interesting part.
//
// The rule used to be "no blob hash at all", because content addressing would make a URL a forwardable,
// unrevokable capability. The maintainer narrowed it (option A) so that a photograph could have a durable public
// address at all — tasks 447 and 456 — to: a ref may appear **where the route resolving it is album-scoped and
// publication-checked.** `albumItemRef` is, and `TestAlbumMediaByRefIsScopedToItsPublishedAlbum` is what holds
// that: one album's ref 404s in another and in the unpublished one, so unpublishing revokes it and a hash on its
// own reaches nothing.
//
// What did **not** change, and is what this test now guards: the **display** ref is the address, and the
// thumbnail's and the 800px rendition's hashes still appear nowhere. They are derived, a visitor has no reason
// to name one, and the variant is a query parameter on the same address — so there is no honest reason for a
// second or third hash to be in the page, and "adding a rendition" remains exactly the moment somebody would
// pass one through to the template.
//
// `glimtpublic_test.go` keeps the **unnarrowed** rule for glimt, where media is per-member scoped and a hash
// really would be a bearer token. That is the same shape as every other exception in this codebase: narrowed
// where it was argued, untouched everywhere else.
func TestTheAlbumPageNeverPutsADerivedRenditionRefInItsHTML(t *testing.T) {
	_, item, srv := mediumAlbumApp(t)

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	for name, ref := range map[string]string{
		"medium": item.MediumRef,
		"thumb":  item.ThumbRef,
	} {
		if ref == "" {
			t.Fatalf("the fixture has no %s rendition, so this test is asserting nothing", name)
		}
		if strings.Contains(page, ref) {
			t.Errorf("the %s rendition's content hash appears in the album page's HTML. Only the display ref "+
				"is an address; a derived rendition is reached by ?variant= on that same address, so a second "+
				"hash in the page is a hash in circulation for no reason", name)
		}
	}

	// And the display ref *is* there, which is what makes the two halves of this rule distinguishable rather
	// than a matter of which one somebody remembered.
	if !strings.Contains(page, item.Ref) {
		t.Errorf("the display ref must be in the page: it is the photograph's address (task 447)\n%s", page)
	}
}

// The ingest produces both derived renditions, and stores both as **cache** rather than as originals.
func TestAlbumIngestProducesBothDerivedRenditions(t *testing.T) {
	app := newTestApp(t)
	store := newClassStore(app.blobs)
	app.blobs = store

	prepared, err := app.storeAlbumImage(context.Background(), app.config.eventYear, testImage(t, 2000, 1500))
	if err != nil {
		t.Fatalf("storeAlbumImage: %v", err)
	}
	if prepared.MediumRef == "" {
		t.Fatal("no medium rendition was produced")
	}
	if prepared.MediumRef == prepared.ThumbRef || prepared.MediumRef == prepared.Ref {
		t.Fatal("the medium rendition is the same object as another rendition, so it is not a third size")
	}

	// The sizes, from the bytes rather than from the names.
	for name, c := range map[string]struct {
		ref  string
		want int
	}{
		"full":   {prepared.Ref, maxGlimtEdge},
		"medium": {prepared.MediumRef, mediumEdge},
		"thumb":  {prepared.ThumbRef, glimtThumbEdges[0]},
	} {
		rc, err := app.blobs.Get(context.Background(), blob.Ref(c.ref))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var buf bytes.Buffer
		buf.ReadFrom(rc)
		rc.Close()
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("%s is not a decodable JPEG: %v", name, err)
		}
		if cfg.Width != c.want {
			t.Errorf("%s rendition is %dpx wide, want %dpx", name, cfg.Width, c.want)
		}
	}

	// Derived, so outside the backup scope (task 429) and rebuildable (task 430).
	if !store.cached[blob.Ref(prepared.MediumRef)] {
		t.Error("the medium rendition was stored as an original, which puts a regenerable 800px copy of " +
			"every photograph into the backup")
	}
	// Since PRD 027 R4, so is the 1600px display image: the photographer's file is kept now, so this is no longer
	// anybody's only copy. The classification follows "is this the only copy of these pixels", which is why the
	// original below is the one that must **not** be cache — and why a glimt's display image still is not (R4a).
	if !store.cached[blob.Ref(prepared.Ref)] {
		t.Error("the 1600px display image was stored as an original; since PRD 027 it is derived from the " +
			"photographer's file and belongs in the cache class")
	}
	if prepared.Original == nil || store.cached[blob.Ref(prepared.Original.Ref)] {
		t.Error("the photographer's original is missing or was stored as cache; it is the only copy of those " +
			"pixels and cannot be rebuilt from anything")
	}
}

// Renditions are looked up by name, never by index.
//
// `prepared.Thumbs` is in the order of `libraryThumbEdges`, so an index silently depends on that order —
// and adding a size re-points every existing reader. That is a data bug that presents as a layout bug: the
// grid starts serving 800px tiles and nothing errors.
func TestRenditionsAreResolvedByNameNotByIndex(t *testing.T) {
	src, err := os.ReadFile("albummedia.go")
	if err != nil {
		t.Fatalf("reading albummedia.go: %v", err)
	}
	if strings.Contains(string(src), "prepared.Thumbs[0]") {
		t.Error("albummedia.go indexes prepared.Thumbs. Use imaging.ThumbName and match on the rendition's " +
			"name, or adding a size re-points the thumbnail to whatever now sits at [0]")
	}
}

// Glimt's storage must not change as a side effect of the library gaining a rendition.
//
// PRD 023 §11 Q8 asked whether glimt should get the 800px rendition too; the answer belongs to PRD 019 and
// is deliberately **not** taken here. `glimtThumbEdges` was shared, so editing it would have changed a
// second feature's storage for every future upload, silently, in a diff about album pages.
func TestGlimtStorageIsUnchangedByTheLibrarysRenditions(t *testing.T) {
	if len(glimtThumbEdges) != 1 {
		t.Errorf("glimtThumbEdges = %v, want exactly one size. Adding the library's 800px rendition here "+
			"would change glimt's storage for every future upload — that decision belongs to PRD 019 "+
			"(PRD 023 §11 Q8), not to a constant edit in a diff about album pages", glimtThumbEdges)
	}

	app := newTestApp(t)
	store := newClassStore(app.blobs)
	app.blobs = store

	stored, err := app.storeGlimtImage(
		httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil),
		testImage(t, 2000, 1500),
	)
	if err != nil {
		t.Fatalf("storeGlimtImage: %v", err)
	}
	// Two objects for a glimt item: the full rendition and one thumbnail. A third would mean glimt silently
	// acquired the library's rendition set.
	if stored.Ref == "" || stored.ThumbRef == "" {
		t.Fatalf("a glimt item should have a full rendition and a thumbnail, got %+v", stored)
	}
	if len(store.cached) != 2 {
		t.Errorf("a glimt upload stored %d objects, want 2 (one original, one cached thumbnail)", len(store.cached))
	}
}

// The library's rendition list must stay in step with the width the viewer advertises.
//
// The viewer names `800w` in its `srcset` (task 410). If `mediumEdge` moved and that literal did not, the
// browser would be told a size the bytes do not have — and it would still pick sensibly enough that nobody
// would notice.
func TestTheViewersAdvertisedWidthMatchesTheStoredRendition(t *testing.T) {
	if mediumEdge != 800 {
		t.Fatalf("mediumEdge = %d; task 410's srcset advertises 800w, so these must move together", mediumEdge)
	}
	js := viewerAsset(t, "viewer.js")
	if !strings.Contains(js, "800w") {
		t.Error("viewer.js no longer advertises an 800w candidate, so the medium rendition is never requested")
	}
	if !strings.Contains(js, "1600w") || maxGlimtEdge != 1600 {
		t.Errorf("the display image is %dpx but viewer.js advertises 1600w; these must agree", maxGlimtEdge)
	}
}
