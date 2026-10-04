package blob

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func stage(t *testing.T, f Files, data []byte) string {
	t.Helper()
	dir, err := f.StagingDir()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "upload-"+filepath.Base(t.Name()))
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPutFileAdoptsAndHashes(t *testing.T) {
	for name, s := range map[string]interface {
		Store
		Files
	}{"file": mustFileStore(t), "memory": NewMemoryStore()} {
		t.Run(name, func(t *testing.T) {
			data := bytes.Repeat([]byte("video"), 1000)
			p := stage(t, s, data)
			ref, err := s.PutFile(context.Background(), p, false)
			if err != nil {
				t.Fatal(err)
			}
			if ref != ComputeRef(data) {
				t.Errorf("ref %s is not the content hash", ref)
			}
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Error("the staged file was not consumed")
			}
			rs, err := s.Open(context.Background(), ref)
			if err != nil {
				t.Fatal(err)
			}
			defer rs.Close()
			if _, err := rs.Seek(4995, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			tail, _ := io.ReadAll(rs)
			if string(tail) != "video" {
				t.Errorf("seek read %q", tail)
			}
		})
	}
}

// A staged original whose bytes are only in the cache must be promoted, exactly as Put does.
func TestPutFilePromotesACachedCopy(t *testing.T) {
	s := mustFileStore(t)
	data := []byte("same bytes")
	if _, err := s.PutCache(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	ref, err := s.PutFile(context.Background(), stage(t, s, data), false)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.path(originalDir, ref)
	if _, err := os.Stat(p); err != nil {
		t.Errorf("the original was left only in the cache: %v", err)
	}
}

func TestLocalPathNamesTheObject(t *testing.T) {
	s := mustFileStore(t)
	ref, _ := s.Put(context.Background(), []byte("x"))
	p, ok, err := s.LocalPath(context.Background(), ref)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if b, _ := os.ReadFile(p); string(b) != "x" {
		t.Errorf("path holds %q", b)
	}
}

func mustFileStore(t *testing.T) *FileStore {
	t.Helper()
	s, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}
