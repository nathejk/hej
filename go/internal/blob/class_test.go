package blob

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Originals and derived renditions must be distinguishable **on the volume**, because the
// consumer of the distinction is a backup command and not this process. See the package
// comment, and task 384's outstanding criterion: a backup target confirmed to hold the
// photograph library, with a restore spot-checked.
//
// These tests are deliberately about the filesystem rather than about a method's return
// value. A split that existed only in Go would satisfy an interface and protect nothing.

func newStore(t *testing.T) *FileStore {
	t.Helper()
	s, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return s
}

// under reports whether the object is present in the given class subtree.
func under(t *testing.T, dir string, ref Ref) bool {
	t.Helper()
	name := string(ref)
	_, err := os.Stat(filepath.Join(dir, name[:2], name))
	return err == nil
}

// statObject stats an object inside a class subtree, by its full ref name.
func statObject(dir, name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(dir, name[:2], name))
}

// tempFiles lists any leftover write-in-progress files in a bucket.
func tempFiles(t *testing.T, dir, bucket string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, bucket))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestPutStoresAnOriginalAndPutCacheDoesNot(t *testing.T) {
	s := newStore(t)

	full, err := s.Put(t.Context(), []byte("the photograph"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	thumb, err := s.PutCache(t.Context(), []byte("its thumbnail"))
	if err != nil {
		t.Fatalf("PutCache: %v", err)
	}

	if !under(t, s.OriginalDir(), full) {
		t.Errorf("a photograph stored with Put is not under %s, so a backup of that path would "+
			"miss it — which is the one failure this layout exists to prevent", s.OriginalDir())
	}
	if under(t, s.CacheDir(), full) {
		t.Errorf("a photograph stored with Put appeared under %s, which a backup may skip", s.CacheDir())
	}
	if !under(t, s.CacheDir(), thumb) {
		t.Errorf("a thumbnail stored with PutCache is not under %s", s.CacheDir())
	}
	if under(t, s.OriginalDir(), thumb) {
		t.Errorf("a thumbnail appeared under %s, which puts regenerable bytes in the backup and "+
			"makes the backup grow with every rendition we ever add", s.OriginalDir())
	}
}

// The property an operator actually depends on, stated as one assertion: excluding the
// cache directory loses no original.
func TestExcludingTheCacheDirectoryLosesNoOriginal(t *testing.T) {
	s := newStore(t)

	keep, err := s.Put(t.Context(), []byte("a photograph nobody else has"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := s.PutCache(t.Context(), []byte("a rendition of it")); err != nil {
		t.Fatalf("PutCache: %v", err)
	}

	// What a restore-from-backup looks like: the cache subtree was never copied.
	if err := os.RemoveAll(s.CacheDir()); err != nil {
		t.Fatalf("simulating a restore without the cache: %v", err)
	}

	if ok, err := s.Exists(t.Context(), keep); err != nil || !ok {
		t.Fatalf("the original did not survive a restore that skipped the cache (exists=%v, err=%v)", ok, err)
	}
	rc, err := s.Get(t.Context(), keep)
	if err != nil {
		t.Fatalf("Get after the restore: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "a photograph nobody else has" {
		t.Errorf("the original's bytes changed across the restore: %q", got)
	}
}

// Readers are class-agnostic on purpose: a Ref travels through projections and URLs and
// says nothing about how the bytes were made. If Get could only find originals, every
// thumbnail in the service would 404 — and the projections would need a second column to
// prevent it.
func TestReadsAndDeletesDoNotNeedToKnowTheClass(t *testing.T) {
	s := newStore(t)

	thumb, err := s.PutCache(t.Context(), []byte("a thumbnail"))
	if err != nil {
		t.Fatalf("PutCache: %v", err)
	}

	if ok, err := s.Exists(t.Context(), thumb); err != nil || !ok {
		t.Fatalf("Exists could not find a cached object (exists=%v, err=%v)", ok, err)
	}
	rc, err := s.Get(t.Context(), thumb)
	if err != nil {
		t.Fatalf("Get could not find a cached object: %v", err)
	}
	rc.Close()

	if err := s.Delete(t.Context(), thumb); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if ok, _ := s.Exists(t.Context(), thumb); ok {
		t.Error("Delete left a cached object behind")
	}
}

// A purge has established that nothing references these bytes. A copy left in the other
// subtree would be a deleted photograph still on disk, and still in the backup.
func TestDeleteRemovesBothCopies(t *testing.T) {
	s := newStore(t)
	data := []byte("bytes that ended up in both classes")

	ref, err := s.Put(t.Context(), data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Force the pathological case directly, since dedup normally prevents it.
	name := string(ref)
	dst := filepath.Join(s.CacheDir(), name[:2])
	if err := os.MkdirAll(dst, 0o700); err != nil {
		t.Fatalf("seeding a second copy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, name), data, 0o600); err != nil {
		t.Fatalf("seeding a second copy: %v", err)
	}

	if err := s.Delete(t.Context(), ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if under(t, s.OriginalDir(), ref) || under(t, s.CacheDir(), ref) {
		t.Error("Delete left a copy behind; a purged photograph must not survive in either class")
	}
}

// Dedup spans both classes, so identical bytes are never stored twice — but only in the
// direction that is safe. See the comment in FileStore.put.
func TestPutCacheReusesAnExistingOriginal(t *testing.T) {
	s := newStore(t)
	data := []byte("bytes that arrive twice")

	ref, err := s.Put(t.Context(), data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := s.PutCache(t.Context(), data); err != nil {
		t.Fatalf("PutCache: %v", err)
	}

	if under(t, s.CacheDir(), ref) {
		t.Error("PutCache wrote a second copy of bytes already held as an original")
	}
	if !under(t, s.OriginalDir(), ref) {
		t.Error("PutCache must not move an original into the cache")
	}
}

// The costly direction, and the reason dedup is not symmetric: bytes held only as cache
// are in the subtree a backup skips. A caller declaring them an original must be able to
// change that, or the sole copy of a photograph sits outside the backup.
//
// It is a narrow case — a full rendition and its thumbnail do not hash alike — but it is
// reachable with an image small enough that resizing is a no-op, and it fails silently.
func TestPutPromotesBytesHeldOnlyAsCache(t *testing.T) {
	s := newStore(t)
	data := []byte("an image too small to resize")

	ref, err := s.PutCache(t.Context(), data)
	if err != nil {
		t.Fatalf("PutCache: %v", err)
	}
	if _, err := s.Put(t.Context(), data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if !under(t, s.OriginalDir(), ref) {
		t.Errorf("Put returned a ref whose only copy is under %s. A backup of %s would not contain "+
			"it, and nothing would say so", s.CacheDir(), s.OriginalDir())
	}
}

// Objects written before the split have no recorded class, so the migration treats them
// all as originals: over-counting the backup costs disk, guessing wrong costs photographs.
func TestExistingObjectsMigrateIntoTheOriginalSubtree(t *testing.T) {
	root := t.TempDir()

	// The old flat layout: <root>/<ab>/<ref>.
	data := []byte("a photograph from before the split")
	ref := ComputeRef(data)
	name := string(ref)
	legacy := filepath.Join(root, name[:2])
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatalf("seeding the legacy layout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacy, name), data, 0o600); err != nil {
		t.Fatalf("seeding the legacy layout: %v", err)
	}

	s, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	if !under(t, s.OriginalDir(), ref) {
		t.Fatalf("an object from the old layout was not migrated into %s, so the first backup after "+
			"this deploy would silently omit everything uploaded before it", s.OriginalDir())
	}
	if _, err := os.Stat(filepath.Join(legacy, name)); err == nil {
		t.Error("the legacy copy is still at the top level; the object is now stored twice")
	}
	rc, err := s.Get(t.Context(), ref)
	if err != nil {
		t.Fatalf("a migrated object must still be readable by its ref: %v", err)
	}
	defer rc.Close()
	if got, _ := io.ReadAll(rc); string(got) != string(data) {
		t.Errorf("migration changed the bytes: %q", got)
	}
}

// The migration runs on every boot, so it has to be a no-op the second time and has to
// cope with a bucket that exists in both places — which is what an interrupted first run
// leaves behind.
func TestMigrationIsIdempotentAndMergesAPartialRun(t *testing.T) {
	root := t.TempDir()

	older := []byte("migrated on the interrupted run")
	newer := []byte("still at the top level")
	// Same bucket for both, which is the case a plain rename cannot handle. Searched for
	// rather than hoped for: otherwise the test passes on the rename path and never
	// exercises the merge at all.
	if ComputeRef(older)[:2] != ComputeRef(newer)[:2] {
		older, newer = collidingPayloads(t)
	}
	olderRef, newerRef := ComputeRef(older), ComputeRef(newer)
	if olderRef[:2] != newerRef[:2] {
		t.Fatalf("the two payloads must share a bucket or this test silently exercises the plain-rename "+
			"path instead of the merge: %s vs %s", olderRef[:2], newerRef[:2])
	}

	mustWrite(t, filepath.Join(root, originalDir, string(olderRef)[:2]), olderRef, older)
	mustWrite(t, filepath.Join(root, string(newerRef)[:2]), newerRef, newer)
	// And a duplicate of the already-migrated object still at the top level, as an
	// interrupted run would leave it.
	mustWrite(t, filepath.Join(root, string(olderRef)[:2]), olderRef, older)

	s, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	for _, ref := range []Ref{olderRef, newerRef} {
		if !under(t, s.OriginalDir(), ref) {
			t.Errorf("%s was not merged into %s", ref, s.OriginalDir())
		}
		if _, err := os.Stat(filepath.Join(root, string(ref)[:2], string(ref))); err == nil {
			t.Errorf("%s is still at the top level after the merge", ref)
		}
	}

	// Booting again must change nothing and must not fail.
	if _, err := NewFileStore(root); err != nil {
		t.Fatalf("a second boot must be a no-op, got: %v", err)
	}
	if ok, _ := s.Exists(t.Context(), olderRef); !ok {
		t.Error("a second boot lost an object")
	}
}

func mustWrite(t *testing.T, dir string, ref Ref, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, string(ref)), data, 0o600); err != nil {
		t.Fatalf("write %s: %v", ref, err)
	}
}

// collidingPayloads returns two distinct payloads whose refs share a hash bucket. With
// 256 buckets a pair turns up almost immediately.
func collidingPayloads(t *testing.T) (a, b []byte) {
	t.Helper()
	seen := map[string][]byte{}
	for i := 0; i < 10000; i++ {
		payload := []byte("payload-" + strconv.Itoa(i))
		bucket := string(ComputeRef(payload))[:2]
		if first, ok := seen[bucket]; ok {
			return first, payload
		}
		seen[bucket] = payload
	}
	t.Fatal("no two payloads shared a bucket in 10000 tries, which is statistically impossible")
	return nil, nil
}

// Objects keep their 0600 in both classes. The cache holds thumbnails of identifiable
// minors just as surely as the originals do; "reproducible" is not "less sensitive".
func TestCachedObjectsAreJustAsPrivate(t *testing.T) {
	s := newStore(t)

	ref, err := s.PutCache(t.Context(), []byte("a thumbnail of a child"))
	if err != nil {
		t.Fatalf("PutCache: %v", err)
	}
	name := string(ref)
	for _, p := range []string{
		s.CacheDir(),
		filepath.Join(s.CacheDir(), name[:2]),
	} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("Stat %s: %v", p, err)
		}
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("%s has mode %o, want 0700", p, perm)
		}
	}
	info, err := os.Stat(filepath.Join(s.CacheDir(), name[:2], name))
	if err != nil {
		t.Fatalf("Stat object: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("a cached object has mode %o, want 0600", perm)
	}
}
