package main

import (
	"bytes"
	"context"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/album"
)

// Rebuilding a rendition that has gone missing (task 430).
//
// What makes these tests worth writing carefully is that the feature's failure modes are all *quiet*.
// A rebuild that never happens looks like a slightly slow page. A degraded answer cached under the wrong
// ETag looks fine until a year later. A rebuild that writes to the wrong ref destroys a photograph and
// the request that did it returns 200.

// countingStore counts the rebuilds that actually wrote bytes.
type countingStore struct {
	blob.Store
	mu      sync.Mutex
	putAs   int
	pending chan struct{} // when non-nil, PutAs blocks on it, to hold a flight open
}

func (s *countingStore) PutAs(ctx context.Context, ref blob.Ref, data []byte) error {
	if s.pending != nil {
		<-s.pending
	}
	s.mu.Lock()
	s.putAs++
	s.mu.Unlock()
	return s.Store.PutAs(ctx, ref, data)
}

func (s *countingStore) rebuilds() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.putAs
}

// missingThumb sets up a source image plus the ref of a thumbnail that is no longer stored — the state
// an emptied cache or a restore-without-cache leaves behind.
func missingThumb(t *testing.T, app *application) (source, target blob.Ref) {
	t.Helper()

	src, err := app.blobs.Put(context.Background(), testImage(t, 1200, 900))
	if err != nil {
		t.Fatalf("seeding the source: %v", err)
	}
	// A real ref for the thumbnail, then removed: the projection still names it, the bytes are gone.
	tgt, err := app.blobs.PutCache(context.Background(), []byte("the thumbnail that used to be here"))
	if err != nil {
		t.Fatalf("seeding the thumbnail: %v", err)
	}
	if err := app.blobs.Delete(context.Background(), tgt); err != nil {
		t.Fatalf("removing the thumbnail: %v", err)
	}
	return src, tgt
}

func TestARebuiltThumbnailLandsUnderItsRecordedRef(t *testing.T) {
	app := newTestApp(t)
	source, target := missingThumb(t, app)

	plan := renditionRepair{Target: target, Source: source, Edge: 320, Quality: 85}
	if err := app.repairRendition(context.Background(), plan); err != nil {
		t.Fatalf("repairRendition: %v", err)
	}

	// The whole point of the scheme: the ref the projection already holds now serves bytes again, even
	// though those bytes hash to something else entirely.
	rc, err := app.blobs.Get(context.Background(), target)
	if err != nil {
		t.Fatalf("the rebuilt rendition is not readable under its recorded ref: %v", err)
	}
	defer rc.Close()
	var got bytes.Buffer
	if _, err := got.ReadFrom(rc); err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if blob.ComputeRef(got.Bytes()) == target {
		t.Error("the fixture accidentally reproduced the original bytes, so this proves nothing about " +
			"storing under a name rather than a hash")
	}

	cfg, err := jpeg.DecodeConfig(bytes.NewReader(got.Bytes()))
	if err != nil {
		t.Fatalf("the rebuilt rendition is not a decodable JPEG: %v", err)
	}
	if cfg.Width != 320 {
		t.Errorf("rebuilt thumbnail is %dpx wide, want the 320px the plan asked for", cfg.Width)
	}

	// And it is cache, not original: a rebuild must not grow the backup.
	if mem, ok := app.blobs.(*blob.MemoryStore); ok && !mem.IsCached(target) {
		t.Error("a rebuilt rendition was classed as an original, which would put it in the backup")
	}
}

// The single-flight. An album page asks for up to sixty thumbnails and a cold cache answers sixty misses
// at once; without coalescing, that is sixty concurrent decodes of the same JPEG on the one process that
// is also serving the pages.
func TestConcurrentMissesRebuildOnlyOnce(t *testing.T) {
	app := newTestApp(t)
	source, target := missingThumb(t, app)

	counting := &countingStore{Store: app.blobs, pending: make(chan struct{})}
	app.blobs = counting

	plan := renditionRepair{Target: target, Source: source, Edge: 320, Quality: 85}

	const callers = 50
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			_ = app.repairRendition(context.Background(), plan)
		}()
	}
	// Let them all pile onto the same flight before any write completes, which is the situation being
	// tested rather than one the scheduler might or might not produce.
	time.Sleep(50 * time.Millisecond)
	close(counting.pending)
	wg.Wait()

	if got := counting.rebuilds(); got != 1 {
		t.Errorf("%d callers caused %d rebuilds, want 1. Sixty tiles of a cold album page would each "+
			"decode and resize the same image", callers, got)
	}
}

// A second rebuild of something already rebuilt must not redo the work. Sequential rather than
// concurrent, so it exercises the Exists re-check inside the flight rather than the flight itself.
func TestARebuildIsNotRepeatedOnceItHasSucceeded(t *testing.T) {
	app := newTestApp(t)
	source, target := missingThumb(t, app)

	counting := &countingStore{Store: app.blobs}
	app.blobs = counting
	plan := renditionRepair{Target: target, Source: source, Edge: 320, Quality: 85}

	for i := 0; i < 5; i++ {
		if err := app.repairRendition(context.Background(), plan); err != nil {
			t.Fatalf("repairRendition %d: %v", i, err)
		}
	}
	if got := counting.rebuilds(); got != 1 {
		t.Errorf("five sequential repairs did %d rebuilds, want 1", got)
	}
}

// A plan is refused rather than acted on when there is nothing to rebuild from, or when the target and
// the source are the same object. The latter is the dangerous one: it is what a swapped pair of
// arguments looks like, and acting on it would mean writing a thumbnail over a photograph.
func TestAnImpossiblePlanIsNeverActedOn(t *testing.T) {
	app := newTestApp(t)
	source, target := missingThumb(t, app)

	for name, plan := range map[string]renditionRepair{
		"no target":            {Source: source, Edge: 320, Quality: 85},
		"no source":            {Target: target, Edge: 320, Quality: 85},
		"target is the source": {Target: source, Source: source, Edge: 320, Quality: 85},
		"no edge":              {Target: target, Source: source, Quality: 85},
		"no quality":           {Target: target, Source: source, Edge: 320},
	} {
		if plan.possible() {
			t.Errorf("%s: plan reports itself possible", name)
		}
		if err := app.repairRendition(context.Background(), plan); err == nil {
			t.Errorf("%s: repairRendition accepted it", name)
		}
	}

	// The source must still be exactly what it was.
	rc, err := app.blobs.Get(context.Background(), source)
	if err != nil {
		t.Fatalf("the source is gone: %v", err)
	}
	defer rc.Close()
	var got bytes.Buffer
	got.ReadFrom(rc)
	if blob.ComputeRef(got.Bytes()) != source {
		t.Error("the source photograph's bytes were modified by a refused repair")
	}
}

// albumAppWithRealImages rebuilds the album fixture with decodable JPEGs, since the shared fixture uses
// text placeholders and a rebuild has to actually decode its source.
func albumAppWithRealImages(t *testing.T) (*application, blob.Ref, blob.Ref, *httptest.Server) {
	t.Helper()

	app, store := albumApp(t)

	full, err := app.blobs.Put(context.Background(), testImage(t, 1200, 900))
	if err != nil {
		t.Fatalf("seeding the full rendition: %v", err)
	}
	thumb, err := app.blobs.PutCache(context.Background(), []byte("a thumbnail about to go missing"))
	if err != nil {
		t.Fatalf("seeding the thumbnail: %v", err)
	}
	store.albums[0].items = []album.Item{{
		Ordinal: 0, Ref: full.String(), ThumbRef: thumb.String(),
		Width: 1200, Height: 900, BoundsVerdict: album.BoundsNone,
	}}

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return app, full, thumb, srv
}

// The degraded answer, and the reason it is safe to give one.
//
// Serving the source in place of the thumbnail is only acceptable if nothing caches those bytes *as* the
// thumbnail. The media routes otherwise answer `immutable` with a year's max-age, so an unlucky client
// arriving during the rebuild window would keep a multi-megabyte image under the thumbnail's URL for a
// year, on a page that shows sixty of them.
func TestTheDegradedAnswerServesTheSourceWithoutPoisoningCaches(t *testing.T) {
	app, full, thumb, srv := albumAppWithRealImages(t)

	if err := app.blobs.Delete(context.Background(), thumb); err != nil {
		t.Fatalf("removing the thumbnail: %v", err)
	}

	resp, body := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0?variant=thumb", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("a missing but rebuildable thumbnail must still answer, got %d", resp.StatusCode)
	}
	_ = body

	// The ETag names the bytes actually sent, not the rendition that was asked for.
	if got := resp.Header.Get("ETag"); !strings.Contains(got, string(full)) {
		t.Errorf("ETag = %q, want the source's ref %q. An ETag naming the thumbnail while the body is "+
			"the full image tells every cache the two are the same bytes", got, full)
	}
	cache := resp.Header.Get("Cache-Control")
	if strings.Contains(cache, "immutable") {
		t.Errorf("Cache-Control = %q on a degraded answer. `immutable` would make a full-size image "+
			"permanent under the thumbnail's URL", cache)
	}
	if strings.Contains(cache, "31536000") {
		t.Errorf("Cache-Control = %q on a degraded answer, want a short window so the next request "+
			"picks up the rebuilt rendition", cache)
	}
}

// And the rebuild does happen, so the degradation is temporary rather than permanent.
func TestAMissingThumbnailComesBackAfterARequest(t *testing.T) {
	app, _, thumb, srv := albumAppWithRealImages(t)

	if err := app.blobs.Delete(context.Background(), thumb); err != nil {
		t.Fatalf("removing the thumbnail: %v", err)
	}

	resp, _ := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0?variant=thumb", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	// The rebuild is off the request path on purpose, so it is not finished when the response is.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if ok, _ := app.blobs.Exists(context.Background(), thumb); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the thumbnail was not rebuilt within 5s of a request that missed it")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The follow-up request gets the real rendition back: the thumbnail's own ETag, and a cacheable
	// answer rather than the degraded one.
	resp2, _ := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0?variant=thumb", nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("want 200 after the rebuild, got %d", resp2.StatusCode)
	}
	if got := resp2.Header.Get("ETag"); !strings.Contains(got, string(thumb)) {
		t.Errorf("after the rebuild the ETag should be the thumbnail's ref %q, got %q", thumb, got)
	}
	if got := resp2.Header.Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("after the rebuild the answer should be cacheable again, got %q", got)
	}
}

// A missing **original** is not repairable and must not pretend otherwise. There is nothing behind a
// full rendition to rebuild it from, which is precisely why it is the half that gets backed up.
func TestAMissingFullRenditionStill404s(t *testing.T) {
	app, full, _, srv := albumAppWithRealImages(t)

	if err := app.blobs.Delete(context.Background(), full); err != nil {
		t.Fatalf("removing the full rendition: %v", err)
	}

	resp, _ := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("a missing original must degrade to 404 per PRD 008 §8, got %d", resp.StatusCode)
	}
}

// A patrol member's portrait is `no-store`, because that is what keeps spejder faces off devices
// (PRD 007 §8). A degraded answer is a worse reason than usual to weaken it.
func TestTheDegradedPathNeverWeakensNoStore(t *testing.T) {
	if strings.Contains(degradedRenditionCacheControl, "no-store") {
		return // vacuously fine
	}
	// Asserted at the source rather than over HTTP: the patrol portrait route needs a whole authenticated
	// crew session to reach, and what matters is the branch, which is one line.
	src, err := os.ReadFile("photo.go")
	if err != nil {
		t.Fatalf("reading photo.go: %v", err)
	}
	if !strings.Contains(string(src), `if cacheControl != "no-store"`) {
		t.Error("streamPortrait's degraded path must leave `no-store` alone; a spejder's face must not " +
			"become cacheable because a thumbnail went missing (PRD 007 §8)")
	}
}

// The end-to-end rehearsal, against the **real** store rather than the in-memory one.
//
// This is the test that actually settles task 429's claim that `cache/` is expendable, and it is worth
// having separately from all of the above: every other test here runs against `MemoryStore`, so the
// subtree layout, the `PutAs` refusal, the atomic rename and the class of a rebuilt object are all
// exercised by a wrapper that could agree with the production store about everything except the thing
// that matters. `TestTheFileStoreReportsFreeSpace` exists in `internal/blob` for the same reason.
//
// The shape is a restore, or an operator reclaiming disk: **delete the entire cache directory**, then
// ask for a thumbnail.
func TestEmptyingTheWholeCacheDirectoryIsRecoverable(t *testing.T) {
	app, store := albumApp(t)

	fs, err := blob.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	app.blobs = fs

	full, err := fs.Put(context.Background(), testImage(t, 1200, 900))
	if err != nil {
		t.Fatalf("seeding the full rendition: %v", err)
	}
	thumb, err := fs.PutCache(context.Background(), testImage(t, 320, 240))
	if err != nil {
		t.Fatalf("seeding the thumbnail: %v", err)
	}
	store.albums[0].items = []album.Item{{
		Ordinal: 0, Ref: full.String(), ThumbRef: thumb.String(),
		Width: 1200, Height: 900, BoundsVerdict: album.BoundsNone,
	}}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// The restore: `original/` came back, `cache/` did not.
	if err := os.RemoveAll(fs.CacheDir()); err != nil {
		t.Fatalf("emptying the cache: %v", err)
	}
	if ok, _ := fs.Exists(context.Background(), thumb); ok {
		t.Fatal("the thumbnail is still present, so this test is not rehearsing a restore")
	}

	// The page still works, because the original is there.
	resp, _ := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0?variant=thumb", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("an album photograph must still serve after the cache was emptied, got %d", resp.StatusCode)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if ok, _ := fs.Exists(context.Background(), thumb); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the cache did not refill itself after a request. Task 429 excludes cache/ from the " +
				"backup on the grounds that it can be rebuilt; this is that claim")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Rebuilt into the right subtree: a repair must not grow the thing the backup has to hold.
	name := string(thumb)
	if _, err := os.Stat(filepath.Join(fs.CacheDir(), name[:2], name)); err != nil {
		t.Errorf("the rebuilt rendition is not under %s: %v", fs.CacheDir(), err)
	}
	if _, err := os.Stat(filepath.Join(fs.OriginalDir(), name[:2], name)); err == nil {
		t.Errorf("the rebuilt rendition landed in %s, which puts regenerable bytes into the backup",
			fs.OriginalDir())
	}

	// And it is a usable image of the right size, not merely a file of the right name.
	rc, err := fs.Get(context.Background(), thumb)
	if err != nil {
		t.Fatalf("reading the rebuilt thumbnail: %v", err)
	}
	defer rc.Close()
	var got bytes.Buffer
	if _, err := got.ReadFrom(rc); err != nil {
		t.Fatalf("reading the rebuilt thumbnail: %v", err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(got.Bytes()))
	if err != nil {
		t.Fatalf("the rebuilt thumbnail is not a decodable JPEG: %v", err)
	}
	if cfg.Width != glimtThumbEdges[0] {
		t.Errorf("the rebuilt thumbnail is %dpx wide, want %d", cfg.Width, glimtThumbEdges[0])
	}
}
