package blob

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// The regression tests for the first deploy of this split, which served zero photographs.
//
// Two mistakes, and the second is the one that mattered:
//
//  1. `NewFileStore` returned the migration's error. `cmd/api` treats that as "blob store unavailable" and
//     falls back to an **empty in-memory store**, so one un-renameable directory meant no photographs at all.
//  2. `locate` searched only the two new subtrees, which made a *successful* migration a precondition for
//     reading anything. A migration that failed, or got halfway, silently hid objects that were right there.
//
// The fix is that the migration is housekeeping and nothing more: reads look in the legacy location too, so
// the store works whether or not it ever ran. That also covers a case with no migration in it at all — an
// operator restoring an old backup lands objects in exactly this layout.

func TestObjectsAreReadableBeforeTheyAreMigrated(t *testing.T) {
	root := t.TempDir()

	data := []byte("a photograph in the old flat layout")
	ref := ComputeRef(data)
	name := string(ref)

	s, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	// Written after construction, so nothing has migrated it: the state a half-finished or failed migration
	// leaves behind, and the state a restored old backup arrives in.
	mustWrite(t, filepath.Join(root, name[:2]), ref, data)

	if ok, err := s.Exists(t.Context(), ref); err != nil || !ok {
		t.Fatalf("an un-migrated object must still be found (exists=%v, err=%v). This is the bug that served "+
			"zero photographs on the first deploy of the split", ok, err)
	}
	rc, err := s.Get(t.Context(), ref)
	if err != nil {
		t.Fatalf("an un-migrated object must still be readable: %v", err)
	}
	defer rc.Close()
	if got, _ := io.ReadAll(rc); string(got) != string(data) {
		t.Errorf("read back %q", got)
	}
}

// A failed migration must not take the store down with it. The store's job is serving photographs; tidying
// the layout for a backup path is worth strictly less than that.
func TestAFailedMigrationStillYieldsAWorkingStore(t *testing.T) {
	root := t.TempDir()

	data := []byte("a photograph whose bucket cannot be moved")
	ref := ComputeRef(data)
	name := string(ref)
	mustWrite(t, filepath.Join(root, name[:2]), ref, data)

	// A plain file where the `original/` directory needs to be: the bluntest way to make both the rename and
	// the per-file merge fail on any filesystem.
	if err := os.WriteFile(filepath.Join(root, originalDir), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("seeding the obstruction: %v", err)
	}

	s, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore must not fail because the migration did: %v", err)
	}
	if s.MigrationError() == nil {
		t.Error("the migration failure should be reported, so an operator learns those objects are outside " +
			"the backup path")
	}
	if ok, err := s.Exists(t.Context(), ref); err != nil || !ok {
		t.Fatalf("the store must still read objects after a failed migration (exists=%v, err=%v)", ok, err)
	}
}

// A legacy object is an original as far as anything here can tell, so a rebuild must not shadow it.
//
// Subtle because `locate` checks `cache/` **before** the legacy path: without this guard a rebuilt rendition
// written to `cache/` would win, and every read would quietly return a re-encode instead of the photograph.
func TestPutAsRefusesToShadowAnUnmigratedOriginal(t *testing.T) {
	root := t.TempDir()

	photograph := []byte("an un-migrated photograph")
	ref := ComputeRef(photograph)
	name := string(ref)

	s, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	mustWrite(t, filepath.Join(root, name[:2]), ref, photograph)

	if err := s.PutAs(t.Context(), ref, []byte("a thumbnail that would shadow it")); err == nil {
		t.Fatal("PutAs wrote a rendition over an un-migrated original's ref. cache/ is searched before the " +
			"legacy location, so every subsequent read would return the re-encode")
	}

	rc, err := s.Get(t.Context(), ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	if got, _ := io.ReadAll(rc); string(got) != string(photograph) {
		t.Errorf("the photograph's bytes changed: %q", got)
	}
}

// Delete has to reach the legacy location too, or a takedown leaves an un-migrated photograph on disk — and
// in the backup, if the backup was taken before the migration.
func TestDeleteRemovesAnUnmigratedObject(t *testing.T) {
	root := t.TempDir()

	data := []byte("a photograph to take down")
	ref := ComputeRef(data)
	name := string(ref)

	s, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	mustWrite(t, filepath.Join(root, name[:2]), ref, data)

	if err := s.Delete(t.Context(), ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if ok, _ := s.Exists(t.Context(), ref); ok {
		t.Error("a takedown left an un-migrated copy on disk")
	}
}
