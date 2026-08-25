package blob

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// stores returns each implementation, so the contract is tested once rather than
// twice — the point of the interface is that callers cannot tell them apart.
func stores(t *testing.T) map[string]Store {
	t.Helper()
	fs, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return map[string]Store{
		"file":   fs,
		"memory": NewMemoryStore(),
	}
}

func TestPutGetRoundTrip(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			want := []byte("a portrait, notionally")

			ref, err := s.Put(t.Context(), want)
			if err != nil {
				t.Fatalf("Put: %v", err)
			}
			if !ref.Valid() {
				t.Fatalf("Put returned an invalid ref: %q", ref)
			}
			if ref != ComputeRef(want) {
				t.Fatalf("ref is not the content hash: got %q", ref)
			}

			rc, err := s.Get(t.Context(), ref)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			defer rc.Close()
			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if string(got) != string(want) {
				t.Fatalf("want %q, got %q", want, got)
			}
		})
	}
}

// The property the whole design rests on: a projection replay re-publishes the same
// reference, so Put must be a no-op rather than a duplicate or an error.
func TestPutIsIdempotent(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			data := []byte("same bytes twice")

			first, err := s.Put(t.Context(), data)
			if err != nil {
				t.Fatalf("first Put: %v", err)
			}
			second, err := s.Put(t.Context(), data)
			if err != nil {
				t.Fatalf("second Put: %v", err)
			}
			if first != second {
				t.Fatalf("refs differ: %q vs %q", first, second)
			}

			if ms, ok := s.(*MemoryStore); ok && ms.Len() != 1 {
				t.Fatalf("want 1 stored object, got %d", ms.Len())
			}
		})
	}
}

func TestGetMissingReturnsErrNotFound(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ref := ComputeRef([]byte("never stored"))
			if _, err := s.Get(t.Context(), ref); !errors.Is(err, ErrNotFound) {
				t.Fatalf("want ErrNotFound, got %v", err)
			}
			ok, err := s.Exists(t.Context(), ref)
			if err != nil {
				t.Fatalf("Exists: %v", err)
			}
			if ok {
				t.Fatal("want Exists false for a missing object")
			}
		})
	}
}

// Retention jobs (PRDs 003/007) must be safely re-runnable.
func TestDeleteIsIdempotent(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ref, err := s.Put(t.Context(), []byte("to be deleted"))
			if err != nil {
				t.Fatalf("Put: %v", err)
			}
			for i := range 2 {
				if err := s.Delete(t.Context(), ref); err != nil {
					t.Fatalf("Delete #%d: %v", i+1, err)
				}
			}
			if ok, _ := s.Exists(t.Context(), ref); ok {
				t.Fatal("object still present after delete")
			}
		})
	}
}

// A Ref arrives from an event body or a URL, so it is untrusted. A traversal
// attempt must be rejected by validation, not sanitised into something plausible.
func TestRefValidationRejectsTraversal(t *testing.T) {
	bad := []Ref{
		"",
		"../../etc/passwd",
		"not-hex-but-the-right-length-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Ref(strings.Repeat("a", 63)), // one short
		Ref(strings.Repeat("g", 64)), // right length, not hex
	}
	for _, ref := range bad {
		if ref.Valid() {
			t.Fatalf("ref %q should be invalid", string(ref))
		}
	}

	fs, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if _, err := fs.Get(t.Context(), "../../etc/passwd"); err == nil {
		t.Fatal("want an error for a traversal ref")
	}
	if _, err := fs.Exists(t.Context(), "../../etc/passwd"); err == nil {
		t.Fatal("want an error for a traversal ref")
	}
}

// Portraits of minors: the directory must not be world-readable.
func TestFileStorePermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "portraits")
	s, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("Stat root: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("want root mode 0700, got %o", perm)
	}

	ref, err := s.Put(t.Context(), []byte("bytes"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	p, err := s.path(ref)
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat object: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("want object mode 0600, got %o", perm)
	}
}

// The case a Docker volume actually produces: the root already exists, and it is
// world-readable. MkdirAll is a no-op on an existing directory, so without an
// explicit chmod the 0700 intent is silently lost — which is exactly what happened
// with the /blobs volume mount (mode 0755) until this was fixed.
func TestFileStoreTightensAnExistingWorldReadableRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "preexisting")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("pre-create root: %v", err)
	}
	// Confirm the premise, so this test cannot pass for the wrong reason.
	if info, err := os.Stat(root); err != nil {
		t.Fatalf("Stat: %v", err)
	} else if info.Mode().Perm() != 0o755 {
		t.Fatalf("setup failed: root is %o, wanted 0755", info.Mode().Perm())
	}

	if _, err := NewFileStore(root); err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("an existing root must be tightened to 0700, got %o", perm)
	}
}

// No temp files should survive a successful Put: a leaked .tmp-* in the bucket
// would be an object nothing references and nothing cleans up.
func TestFileStoreLeavesNoTempFiles(t *testing.T) {
	root := t.TempDir()
	s, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if _, err := s.Put(t.Context(), []byte("bytes")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), ".tmp-") {
			t.Fatalf("leaked temp file: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

func TestConcurrentPutSameBytes(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			data := []byte("contended bytes")
			var wg sync.WaitGroup
			refs := make([]Ref, 8)
			errs := make([]error, 8)

			for i := range refs {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					refs[i], errs[i] = s.Put(context.Background(), data)
				}(i)
			}
			wg.Wait()

			for i, err := range errs {
				if err != nil {
					t.Fatalf("Put %d: %v", i, err)
				}
				if refs[i] != refs[0] {
					t.Fatalf("ref %d differs: %q vs %q", i, refs[i], refs[0])
				}
			}
		})
	}
}
