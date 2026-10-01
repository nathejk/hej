package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"
	"nathejk.dk/internal/blob"
)

// Which bytes are originals and which are a cache is decided at each call site, not by the
// blob store — the store only honours the decision (see the `blob` package comment). So the
// store's own tests cannot catch the mistake that costs something: a sole-copy photograph
// stored with `PutCache` and therefore silently outside the backup.
//
// These tests are about that decision. The rule they encode, which is the whole of it:
//
//	Cache iff the service could rebuild these bytes from something it still holds.
//
// Not "is it small", and not "is it a thumbnail". A glimt's *full* rendition is already
// downscaled to `maxGlimtEdge` and still an original, because no original was kept behind
// it (glimtmedia.go, "Why no original is kept") — those bytes are the only copy of the
// member's photograph, and PRD 027 R4a confirms that stays true: a participant's photograph from
// the race is not photographer quality and is nonetheless the most original copy there will ever be.
//
// The **library** used to be the same case and is no longer. Since PRD 027 it keeps the photographer's
// file, so its 1600px display image became rebuildable and moved to cache; the original is what is
// backed up now. That is a change of which object the rule selects, not a change in the rule.

// classStore records the class each ref was stored under, so a test can assert on the
// decision a handler made.
//
// A wrapper over the real memory store rather than a fake, so the bytes still land
// somewhere and the surrounding handler keeps working normally.
type classStore struct {
	blob.Store
	cached map[blob.Ref]bool
}

func newClassStore(inner blob.Store) *classStore {
	return &classStore{Store: inner, cached: map[blob.Ref]bool{}}
}

func (s *classStore) Put(ctx context.Context, data []byte) (blob.Ref, error) {
	ref, err := s.Store.Put(ctx, data)
	if err == nil {
		s.cached[ref] = false
	}
	return ref, err
}

func (s *classStore) PutCache(ctx context.Context, data []byte) (blob.Ref, error) {
	ref, err := s.Store.PutCache(ctx, data)
	if err == nil {
		if _, seen := s.cached[ref]; !seen {
			s.cached[ref] = true
		}
	}
	return ref, err
}

// TestGlimtUploadClassifiesItsBytes asserts the full rendition is backed up and the
// thumbnail is not.
func TestGlimtUploadClassifiesItsBytes(t *testing.T) {
	app := photoTestApp(t, &cqrstest.Publisher{}, nil)
	store := newClassStore(app.blobs)
	app.blobs = store

	stored, err := app.storeGlimtImage(
		httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil),
		testImage(t, 2000, 2000),
	)
	if err != nil {
		t.Fatalf("storeGlimtImage: %v", err)
	}
	if stored.ThumbRef == "" {
		t.Fatal("no thumbnail was stored, so this test asserts nothing about the interesting half")
	}

	if store.cached[blob.Ref(stored.Ref)] {
		t.Error("a glimt's full rendition was stored as cache. Glimt keep no original " +
			"(glimtmedia.go), so these bytes are the member's only copy — and a backup that " +
			"skips the cache would not contain them")
	}
	if !store.cached[blob.Ref(stored.ThumbRef)] {
		t.Error("a glimt thumbnail was stored as an original, which puts regenerable bytes into " +
			"the backup for every glimt ever posted")
	}
}

// TestAlbumIngestClassifiesItsBytes is the same assertion for the curated library, and PRD 027 **inverted half of
// it** — which is why the reasoning is restated here rather than the test quietly edited.
//
// Until PRD 027 the library's 1600px display image was an original, correctly: the photographer's file was discarded,
// so those bytes were the only copy of those pixels. That is what the blob store's two classes distinguish — "is this
// the only copy", not "was this re-encoded" — and it is the whole reason the glimt test above still expects `Put` for
// a frame that is also a re-encode (R4a: a participant's photograph from the race is modest quality and is
// nonetheless the most original copy that will ever exist).
//
// Now the library keeps the uploaded file, so the display image stops being the only copy and becomes what it always
// looked like: a rendition, rebuildable from the original, outside the backup scope. The thing that must be in
// `original/` is the photographer's file.
//
// The volumes are the reason this matters either way: PRD 022 §6 puts a hand-in at order 1 GB and §11 Q2 means it is
// never reclaimed — PRD 027 makes that 8–15 GB, so every byte wrongly classified is wrong forever.
func TestAlbumIngestClassifiesItsBytes(t *testing.T) {
	app := newTestApp(t)
	store := newClassStore(app.blobs)
	app.blobs = store

	prepared, err := app.storeAlbumImage(context.Background(), app.config.eventYear, testImage(t, 2000, 2000))
	if err != nil {
		t.Fatalf("storeAlbumImage: %v", err)
	}
	if prepared.ThumbRef == "" {
		t.Fatal("no thumbnail was stored, so this test asserts nothing about the interesting half")
	}
	if prepared.Original == nil || prepared.Original.Ref == "" {
		t.Fatal("no original was stored. Every library upload gets one (PRD 027 R1) — there is no size or " +
			"pixel-count condition — and without it the classification below is asserting the wrong thing")
	}

	// **The photographer's file is the backup scope.** If this ever becomes `PutCache`, a restore brings back the
	// renditions and loses every original the event collected, which is the single worst outcome this split exists
	// to prevent — and it would be invisible until somebody wanted to print something.
	if store.cached[blob.Ref(prepared.Original.Ref)] {
		t.Error("the photographer's original was stored as cache. It is the only copy of those pixels and it " +
			"cannot be rebuilt from anything (PRD 027 R1) — a backup that skips the cache class would not " +
			"contain it")
	}
	// And the derivatives are not. Both are checked, because "everything is an original" would pass a test that only
	// looked at the original.
	if !store.cached[blob.Ref(prepared.Ref)] {
		t.Error("the 1600px display image was stored as an original. Since PRD 027 R4 it is derived from the " +
			"photographer's file, so it belongs in the cache class and task 430 rebuilds it on a miss. Note this " +
			"is the **library's** rule alone: a glimt's display image stays an original (R4a), because for a glimt " +
			"there is no truer copy")
	}
	if !store.cached[blob.Ref(prepared.ThumbRef)] {
		t.Error("a library thumbnail was stored as an original. At PRD 022 §6's volumes this is the " +
			"difference the split exists to make, and task 384's backup criterion depends on it")
	}
}

// TestNoCallSiteCachesTheOnlyCopy is a source-level guard, and it is here because the two
// tests above can only cover the call sites that exist today.
//
// The asymmetry is the point: a *missing* `PutCache` costs backup space, which an operator
// can see. A *wrong* `PutCache` drops the only copy of a photograph out of the backup, and
// that is discovered during a restore. So the requirement is that every `PutCache` call
// carries, nearby, the reason the bytes are reproducible — which is the thing a reviewer
// needs in front of them and the thing a hurried edit omits.
func TestNoCallSiteCachesTheOnlyCopy(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		lines := strings.Split(string(src), "\n")
		for i, line := range lines {
			if !strings.Contains(line, "PutCache(") {
				continue
			}
			// The preceding comment block is where the justification belongs.
			nearby := strings.Join(lines[max(0, i-8):i], "\n")
			if !strings.Contains(nearby, "//") {
				t.Errorf("%s:%d calls PutCache with no comment explaining why these bytes can be "+
					"rebuilt. Cache means excluded from the backup; if the reason is not written "+
					"down, the next edit cannot check it", name, i+1)
			}
		}
	}
}
