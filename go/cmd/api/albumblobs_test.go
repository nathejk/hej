package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// The shared-blob hazard between glimt and albums (PRD 011 §8, task 333).
//
// # The bug these tests exist to prevent
//
// The blob store is content-addressed, so identical bytes are **one object with one ref**. An organizer
// curating an album out of a photograph a participant also posted publicly is the *expected* workflow,
// and it produces one object with two owners.
//
// The glimt delete path checks whether a ref is still in use before deleting the bytes — and until this
// task it asked only the other glimt. So retention would eventually expire the glimt and delete bytes a
// public album page still showed, blanking it months later with nothing connecting the two events.

// stubAlbums answers the one read the delete path needs.
type stubAlbums struct {
	// inUse is the set of refs some live album item references.
	inUse map[string]bool
	err   error

	// asked records the refs each call was given, so a test can assert the check actually ran rather
	// than inferring it from an outcome that could also happen by accident.
	asked [][]string
	// excluded records the exclusion sets, so a test can assert the album removal path names the items
	// it is removing — without that, they report their own bytes as in use and nothing is ever deleted.
	excluded [][]album.ItemKey
}

func (s *stubAlbums) Published(string) ([]album.Album, error) { return nil, nil }

func (s *stubAlbums) BySlug(string, string) (album.Album, []album.Item, bool, error) {
	return album.Album{}, nil, false, nil
}

func (s *stubAlbums) Plottable(string) ([]album.PlottableItem, error) { return nil, nil }

func (s *stubAlbums) RefsInUse(_ string, excluding []album.ItemKey, refs []string) (map[string]bool, error) {
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

// **The regression test for the trap.** A ref an album still uses must survive a glimt deletion.
func TestGlimtDeleteKeepsBytesAnAlbumStillUses(t *testing.T) {
	app, _, _ := publicApp(t)

	shared := strings.Repeat("a", 64)
	orphan := strings.Repeat("b", 64)

	// Both objects exist in the store, as they would after two uploads.
	for _, ref := range []string{shared, orphan} {
		if _, err := app.blobs.Put(context.Background(), []byte("bytes for "+ref)); err != nil {
			t.Fatalf("seeding the blob store: %v", err)
		}
	}
	// Put returns the hash of the content, so the refs above are not the stored ones. Store by content
	// and use the real refs instead — the point of the test is the bookkeeping, not the hashing.
	sharedRef, err := app.blobs.Put(context.Background(), []byte("a photograph in both"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	orphanRef, err := app.blobs.Put(context.Background(), []byte("a photograph in the glimt only"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	albums := &stubAlbums{inUse: map[string]bool{sharedRef.String(): true}}
	app.models.Albums = albums

	app.purgeGlimtBlobs(context.Background(), "g-1",
		[]string{sharedRef.String(), orphanRef.String()})

	if exists, _ := app.blobs.Exists(context.Background(), sharedRef); !exists {
		t.Error("the shared object was deleted: an album page that used it is now blank")
	}
	if exists, _ := app.blobs.Exists(context.Background(), orphanRef); exists {
		t.Error("the unshared object should have been deleted")
	}
	if len(albums.asked) != 1 {
		t.Errorf("the album check must run exactly once per purge, ran %d times", len(albums.asked))
	}
}

// If the album read fails we cannot tell whether the bytes are shared, so nothing is deleted. Leaking
// disk is recoverable; blanking a public page is not.
func TestGlimtDeleteKeepsEverythingWhenTheAlbumCheckFails(t *testing.T) {
	app, _, _ := publicApp(t)
	app.models.Albums = &stubAlbums{err: errors.New("database is down")}

	ref, err := app.blobs.Put(context.Background(), []byte("a photograph"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	app.purgeGlimtBlobs(context.Background(), "g-1", []string{ref.String()})

	if exists, _ := app.blobs.Exists(context.Background(), ref); !exists {
		t.Fatal("an unanswerable sharing question must leave the objects in place")
	}
}

// No album projection is not the same as a failing one: with no albums there is nothing to protect, so
// the purge proceeds. Conflating the two would mean a database-free run never frees disk.
func TestGlimtDeleteProceedsWithNoAlbumProjection(t *testing.T) {
	app, _, _ := publicApp(t)
	app.models.Albums = nil

	ref, err := app.blobs.Put(context.Background(), []byte("a photograph"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	app.purgeGlimtBlobs(context.Background(), "g-1", []string{ref.String()})

	if exists, _ := app.blobs.Exists(context.Background(), ref); exists {
		t.Fatal("with no albums to protect, the object should have been deleted")
	}
}

// A thumbnail is as shareable as the image, so it counts as a use.
func TestAlbumRefCheckCoversThumbnails(t *testing.T) {
	app, _, _ := publicApp(t)

	thumbRef, err := app.blobs.Put(context.Background(), []byte("a thumbnail in both"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	app.models.Albums = &stubAlbums{inUse: map[string]bool{thumbRef.String(): true}}

	app.purgeGlimtBlobs(context.Background(), "g-1", []string{thumbRef.String()})

	if exists, _ := app.blobs.Exists(context.Background(), thumbRef); !exists {
		t.Fatal("a shared thumbnail must survive too")
	}
}

// The glimt check and the album check must both be consulted, and a ref in use by *either* kept. This
// guards the union rather than one branch of it.
func TestRefsUsedElsewhereUnionsBothSources(t *testing.T) {
	app, store, _ := publicApp(t)

	glimtShared := strings.Repeat("c", 64)
	albumShared := strings.Repeat("d", 64)
	unshared := strings.Repeat("e", 64)

	// The glimt stub answers from the rows it holds, so give it a second glimt using glimtShared.
	store.rows[0].Media[0].Ref = glimtShared
	app.models.Albums = &stubAlbums{inUse: map[string]bool{albumShared: true}}

	inUse, err := app.refsUsedElsewhere("g-other", []string{glimtShared, albumShared, unshared})
	if err != nil {
		t.Fatalf("refsUsedElsewhere: %v", err)
	}

	if !inUse[glimtShared] {
		t.Error("a ref another glimt uses must be reported in use")
	}
	if !inUse[albumShared] {
		t.Error("a ref an album uses must be reported in use")
	}
	if inUse[unshared] {
		t.Error("a ref nothing uses must not be reported in use")
	}
}
