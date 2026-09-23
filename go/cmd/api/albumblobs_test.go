package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/photo"
)

// The shared-blob hazard between glimt and our own photographs (PRD 011 §8, task 333; PRD 022 §8.3,
// tasks 364/368).
//
// # The bug these tests exist to prevent
//
// The blob store is content-addressed, so identical bytes are **one object with one ref**. An organizer
// curating an album out of a photograph a participant also posted publicly is the *expected* workflow,
// and it produces one object with two owners.
//
// The glimt delete path checks whether a ref is still in use before deleting the bytes — and until task
// 333 it asked only the other glimt. So retention would eventually expire the glimt and delete bytes a
// public album page still showed, blanking it months later with nothing connecting the two events.
//
// # What PRD 022 changed here
//
// The second owner used to be `album`, because an album item carried the refs. After the library split it
// is `photo`, and the swap was **not** cosmetic: an album-side check answers the narrower question "which
// refs are used by photographs that are in an album", and narrower is the dangerous direction for a purge.
// A photographer's upload that no curator has arranged yet would have been reported unused and deleted by
// an unrelated glimt takedown. `TestGlimtDeleteKeepsBytesOfAPhotographInNoAlbum` is that regression.

// stubPhotos answers the one read the delete path needs.
type stubPhotos struct {
	// inUse is the set of refs some live library photograph references.
	inUse map[string]bool
	err   error

	// asked records the refs each call was given, so a test can assert the check actually ran rather
	// than inferring it from an outcome that could also happen by accident.
	asked [][]string
	// excluded records the exclusion sets, so a test can assert a delete path names the photograph it is
	// deleting — without that, it reports its own bytes as in use and nothing is ever deleted.
	excluded [][]string
}

func (s *stubPhotos) Get(string, string) (photo.Photo, bool, error) {
	return photo.Photo{}, false, nil
}

func (s *stubPhotos) RefsInUse(_ string, excluding []string, refs []string) (map[string]bool, error) {
	s.asked = append(s.asked, refs)
	s.excluded = append(s.excluded, excluding)
	if s.err != nil {
		return nil, s.err
	}
	out := map[string]bool{}
	for _, ref := range refs {
		if s.inUse[ref] {
			out[ref] = true
		}
	}
	return out, nil
}

// **The regression test for the trap.** A ref a curated photograph still uses must survive a glimt
// deletion.
func TestGlimtDeleteKeepsBytesAnAlbumStillUses(t *testing.T) {
	app, _, _ := publicApp(t)

	// Put returns the hash of the content, so store by content and use the real refs — the point of the
	// test is the bookkeeping, not the hashing.
	sharedRef, err := app.blobs.Put(context.Background(), []byte("a photograph in both"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	orphanRef, err := app.blobs.Put(context.Background(), []byte("a photograph in the glimt only"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	photos := &stubPhotos{inUse: map[string]bool{sharedRef.String(): true}}
	app.models.Photos = photos

	app.purgeGlimtBlobs(context.Background(), "g-1",
		[]string{sharedRef.String(), orphanRef.String()})

	if exists, _ := app.blobs.Exists(context.Background(), sharedRef); !exists {
		t.Error("the shared object was deleted: an album page that used it is now blank")
	}
	if exists, _ := app.blobs.Exists(context.Background(), orphanRef); exists {
		t.Error("the unshared object should have been deleted")
	}
	if len(photos.asked) != 1 {
		t.Errorf("the library check must run exactly once per purge, ran %d times", len(photos.asked))
	}
}

// The bug the PRD 022 refactor could have introduced, locked out.
//
// A photographer uploads; nobody has put the photograph in an album yet; a participant had posted the same
// image as a glimt and it is now deleted. The photograph is in the library and in no album, so a check
// that went through `album_item` would report its bytes unused and delete them — destroying a
// photographer's work through a path that has nothing to do with them.
//
// This is why `album.RefsInUse` was **removed** rather than reimplemented as a join when the media columns
// moved. The stub here holds the ref while having no album at all.
func TestGlimtDeleteKeepsBytesOfAPhotographInNoAlbum(t *testing.T) {
	app, _, _ := publicApp(t)

	ref, err := app.blobs.Put(context.Background(), []byte("uploaded, not yet curated"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	// The library knows this ref. There is deliberately no album projection at all.
	app.models.Photos = &stubPhotos{inUse: map[string]bool{ref.String(): true}}
	app.models.Albums = nil

	app.purgeGlimtBlobs(context.Background(), "g-1", []string{ref.String()})

	if exists, _ := app.blobs.Exists(context.Background(), ref); !exists {
		t.Fatal("a library photograph that is in no album still owns its bytes")
	}
}

// If the library read fails we cannot tell whether the bytes are shared, so nothing is deleted. Leaking
// disk is recoverable; blanking a public page is not.
func TestGlimtDeleteKeepsEverythingWhenTheAlbumCheckFails(t *testing.T) {
	app, _, _ := publicApp(t)
	app.models.Photos = &stubPhotos{err: errors.New("database is down")}

	ref, err := app.blobs.Put(context.Background(), []byte("a photograph"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	app.purgeGlimtBlobs(context.Background(), "g-1", []string{ref.String()})

	if exists, _ := app.blobs.Exists(context.Background(), ref); !exists {
		t.Fatal("an unanswerable sharing question must leave the objects in place")
	}
}

// No library projection is not the same as a failing one: with no photographs there is nothing to protect,
// so the purge proceeds. Conflating the two would mean a database-free run never frees disk.
func TestGlimtDeleteProceedsWithNoAlbumProjection(t *testing.T) {
	app, _, _ := publicApp(t)
	app.models.Photos = nil
	app.models.Albums = nil

	ref, err := app.blobs.Put(context.Background(), []byte("a photograph"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	app.purgeGlimtBlobs(context.Background(), "g-1", []string{ref.String()})

	if exists, _ := app.blobs.Exists(context.Background(), ref); exists {
		t.Fatal("with no photographs to protect, the object should have been deleted")
	}
}

// A thumbnail is as shareable as the image, so it counts as a use.
func TestAlbumRefCheckCoversThumbnails(t *testing.T) {
	app, _, _ := publicApp(t)

	thumbRef, err := app.blobs.Put(context.Background(), []byte("a thumbnail in both"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	app.models.Photos = &stubPhotos{inUse: map[string]bool{thumbRef.String(): true}}

	app.purgeGlimtBlobs(context.Background(), "g-1", []string{thumbRef.String()})

	if exists, _ := app.blobs.Exists(context.Background(), thumbRef); !exists {
		t.Fatal("a shared thumbnail must survive too")
	}
}

// The glimt check and the library check must both be consulted, and a ref in use by *either* kept. This
// guards the union rather than one branch of it.
func TestRefsUsedElsewhereUnionsBothSources(t *testing.T) {
	app, store, _ := publicApp(t)

	glimtShared := strings.Repeat("c", 64)
	photoShared := strings.Repeat("d", 64)
	unshared := strings.Repeat("e", 64)

	// The glimt stub answers from the rows it holds, so give it a second glimt using glimtShared.
	store.rows[0].Media[0].Ref = glimtShared
	app.models.Photos = &stubPhotos{inUse: map[string]bool{photoShared: true}}

	inUse, err := app.refsUsedElsewhere("g-other", []string{glimtShared, photoShared, unshared})
	if err != nil {
		t.Fatalf("refsUsedElsewhere: %v", err)
	}

	if !inUse[glimtShared] {
		t.Error("a ref another glimt uses must be reported in use")
	}
	if !inUse[photoShared] {
		t.Error("a ref a library photograph uses must be reported in use")
	}
	if inUse[unshared] {
		t.Error("a ref nothing uses must not be reported in use")
	}
}

// The glimt delete path deletes no photograph, so it must not exclude one from the check. Passing a
// non-empty exclusion here would mean a glimt takedown could delete bytes a photograph still owns.
func TestGlimtDeleteExcludesNoPhotograph(t *testing.T) {
	app, _, _ := publicApp(t)
	photos := &stubPhotos{}
	app.models.Photos = photos

	ref, err := app.blobs.Put(context.Background(), []byte("a photograph"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	app.purgeGlimtBlobs(context.Background(), "g-1", []string{ref.String()})

	if len(photos.excluded) != 1 {
		t.Fatalf("want one call, got %d", len(photos.excluded))
	}
	if len(photos.excluded[0]) != 0 {
		t.Errorf("a glimt deletion must exclude no photograph, got %v", photos.excluded[0])
	}
}
