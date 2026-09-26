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
// member's photograph. Same for an admin upload: PRD 022 §8.5 keeps no separate original,
// and §11 Q2 says the photograph is never purged.

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

// TestAlbumIngestClassifiesItsBytes is the same assertion for the curated library, which is
// the one whose volume actually motivated the split: PRD 022 §6 puts a hand-in at order
// 1 GB, and §11 Q2 means it is never reclaimed.
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

	if store.cached[blob.Ref(prepared.Ref)] {
		t.Error("a library photograph's full rendition was stored as cache. An admin upload keeps " +
			"no separate original (PRD 022 §8.5), so this is the photographer's only copy")
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
