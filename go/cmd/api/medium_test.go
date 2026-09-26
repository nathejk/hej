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

// The tile offers `data-medium` **only** when the photograph has one.
//
// An attribute that was always present would name a URL the server answers by falling back — correct bytes
// under an `800w` label that is a lie, which is the one thing a `srcset` candidate must not be. The viewer
// then emits no `srcset` at all for such a photograph (task 410).
func TestTheAlbumTileOffersTheMediumRenditionOnlyWhenItExists(t *testing.T) {
	_, _, srv := mediumAlbumApp(t)

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)
	if !strings.Contains(page, `data-medium="/api/public/albums/al-1/media/0?variant=medium"`) {
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

// No blob hash may appear in a public payload — content addressing would make it a forwardable, unrevokable
// capability. So the view model carries a **boolean**, not the ref, and the page addresses photographs by
// ordinal. This is the rule `glimtpublic_test.go` asserts structurally; restated here because adding a
// rendition is exactly the moment somebody would pass the ref through to the template.
func TestTheAlbumPageNeverPutsARenditionRefInItsHTML(t *testing.T) {
	_, item, srv := mediumAlbumApp(t)

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)
	for name, ref := range map[string]string{
		"full":   item.Ref,
		"medium": item.MediumRef,
		"thumb":  item.ThumbRef,
	} {
		if strings.Contains(page, ref) {
			t.Errorf("the %s rendition's content hash appears in the album page's HTML. A hash in a public "+
				"response is a forwardable, unrevokable capability — the page addresses photographs by "+
				"ordinal for exactly this reason", name)
		}
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
	if store.cached[blob.Ref(prepared.Ref)] {
		t.Error("the full rendition was stored as cache; it is the photographer's only copy")
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
