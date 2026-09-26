package blob

import (
	"io"
	"strings"
	"testing"
)

// Rebuilding a lost rendition under the ref that was already recorded (task 430).
//
// The scheme's soundness rests entirely on one boundary: a derived ref may be treated as
// a name, an original's ref may not. These tests are mostly about that boundary, because
// the feature working is easy and the boundary holding is what makes it safe.

func TestPutAsRebuildsARenditionUnderItsExistingRef(t *testing.T) {
	s := newStore(t)

	// A rendition as it was first stored, then lost — an emptied cache, or a restore that
	// omitted it.
	ref, err := s.PutCache(t.Context(), []byte("the thumbnail as first encoded"))
	if err != nil {
		t.Fatalf("PutCache: %v", err)
	}
	if err := s.Delete(t.Context(), ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Rebuilt from the original: same pixels, same recipe, different bytes, because a
	// re-encode is not byte-identical. This is the case content addressing cannot express.
	rebuilt := []byte("the thumbnail re-encoded, byte-different")
	if ComputeRef(rebuilt) == ref {
		t.Fatal("the fixture must not hash to the old ref, or it proves nothing")
	}
	if err := s.PutAs(t.Context(), ref, rebuilt); err != nil {
		t.Fatalf("PutAs: %v", err)
	}

	// The whole point: the ref the projections already hold now serves the new bytes, so
	// no row changed and no event was published from a read path.
	rc, err := s.Get(t.Context(), ref)
	if err != nil {
		t.Fatalf("Get after the rebuild: %v", err)
	}
	defer rc.Close()
	if got, _ := io.ReadAll(rc); string(got) != string(rebuilt) {
		t.Errorf("the old ref serves %q, want the rebuilt bytes", got)
	}
	if !under(t, s.CacheDir(), ref) {
		t.Errorf("a rebuilt rendition must land in %s, so it stays out of the backup", s.CacheDir())
	}
	if under(t, s.OriginalDir(), ref) {
		t.Error("a rebuilt rendition must never land in the original subtree")
	}
}

// The guardrail. A repair path holds a source ref and a target ref, and passing them the
// wrong way round is an ordinary mistake — one that would otherwise overwrite the only
// copy of a photograph with a thumbnail of it, in the subtree the backup covers, with the
// checksum property that would have detected it deliberately given up.
//
// This is the single most important test in the file.
func TestPutAsRefusesToOverwriteAnOriginal(t *testing.T) {
	s := newStore(t)

	photograph := []byte("the photographer's only copy")
	ref, err := s.Put(t.Context(), photograph)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	err = s.PutAs(t.Context(), ref, []byte("a thumbnail, passed in the wrong argument"))
	if err == nil {
		t.Fatal("PutAs overwrote an original. Nothing else in this package protects that data: " +
			"its ref is its checksum precisely because it cannot be rebuilt")
	}
	if !strings.Contains(err.Error(), "original") {
		t.Errorf("the refusal should say why, got: %v", err)
	}

	rc, err := s.Get(t.Context(), ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	if got, _ := io.ReadAll(rc); string(got) != string(photograph) {
		t.Errorf("the original's bytes were modified: %q", got)
	}
}

// An original's ref must remain exactly its content hash, forever. That is the property
// that lets the irreplaceable half of the store be verified, and it is the half of the
// bargain that buys the freedom taken with derived refs.
func TestAnOriginalsRefIsAlwaysItsContentHash(t *testing.T) {
	s := newStore(t)

	data := []byte("a photograph")
	ref, err := s.Put(t.Context(), data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if ref != ComputeRef(data) {
		t.Fatalf("Put returned %q, want the content hash %q", ref, ComputeRef(data))
	}

	rc, err := s.Get(t.Context(), ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if ComputeRef(got) != ref {
		t.Error("an original's stored bytes no longer hash to its ref, so nothing can verify them")
	}
}

func TestPutAsRefusesAnInvalidRef(t *testing.T) {
	s := newStore(t)
	if err := s.PutAs(t.Context(), "../../etc/passwd", []byte("x")); err == nil {
		t.Fatal("want an error for a ref that is not a hash, not a file at that path")
	}
}

// A rebuild happens while requests are being served, so a reader must see the old bytes
// or the new ones — never a half-written JPEG. Asserted through the shared write helper
// rather than by racing, which would be flaky: what is being pinned is that PutAs goes
// through the same atomic rename, and so cannot leave a partial object behind.
func TestPutAsIsAtomicAndPrivate(t *testing.T) {
	s := newStore(t)

	ref, err := s.PutCache(t.Context(), []byte("first"))
	if err != nil {
		t.Fatalf("PutCache: %v", err)
	}
	if err := s.PutAs(t.Context(), ref, []byte("second, longer than the first")); err != nil {
		t.Fatalf("PutAs: %v", err)
	}

	name := string(ref)
	info, err := statObject(s.CacheDir(), name)
	if err != nil {
		t.Fatalf("stat the rebuilt object: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("a rebuilt rendition has mode %o, want 0600 — it is a photograph of a child "+
			"just as much as the one it was made from", perm)
	}
	// No temp file survived the write.
	if leftovers := tempFiles(t, s.CacheDir(), name[:2]); len(leftovers) > 0 {
		t.Errorf("PutAs left temp files behind: %v", leftovers)
	}
}

// The memory store must refuse exactly what the file store refuses. It is the store every
// cmd/api test runs against, so a repair path that muddled its two refs would otherwise
// pass the entire suite and fail only in production.
func TestTheMemoryStoreMirrorsThePutAsRefusal(t *testing.T) {
	s := NewMemoryStore()

	ref, err := s.Put(t.Context(), []byte("an original"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := s.PutAs(t.Context(), ref, []byte("a rebuild")); err == nil {
		t.Error("the memory store allowed a rebuild over an original; the file store refuses it, " +
			"and a test store that is more permissive than production hides exactly this bug")
	}

	cached, err := s.PutCache(t.Context(), []byte("a rendition"))
	if err != nil {
		t.Fatalf("PutCache: %v", err)
	}
	if err := s.PutAs(t.Context(), cached, []byte("rebuilt")); err != nil {
		t.Errorf("the memory store refused a legitimate rebuild: %v", err)
	}
	if !s.IsCached(cached) {
		t.Error("a rebuilt rendition must still be classed as cache")
	}
}
