package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Files is the large-object capability (PRD 029): objects too big to hold in memory, and reads that seek.
//
// # An optional capability, not a widening of Store
//
// For FreeSpacer's reason: Store's methods take and return byte slices because every object so far has been a
// photograph, and a 4 GB phone video is not one. Rather than change every caller and every fake, a store that can
// handle files says so, and the video paths type-assert. Both real stores implement it.
type Files interface {
	// StagingDir is a directory **on the same volume** as the objects, where a caller may build a file that
	// PutFile will then adopt with a rename rather than a copy. Outside original/ and cache/, so a half-uploaded
	// file is never in a backup and never mistaken for an object.
	StagingDir() (string, error)

	// PutFile adopts the file at path as an object in the original class, or the cache class when cache is
	// true, hashing it by streaming. The file is consumed: moved into place, or removed when the object already
	// exists. Dedup and promotion follow Put/PutCache exactly.
	PutFile(ctx context.Context, path string, cache bool) (Ref, error)

	// Open returns the object as a seekable reader, which is what HTTP Range serving needs (task 496).
	Open(ctx context.Context, ref Ref) (io.ReadSeekCloser, error)

	// LocalPath returns a filesystem path holding the object's bytes, for tools that read files (ffmpeg).
	// ok is false when the store has no such path; callers then copy from Open into a temporary file.
	LocalPath(ctx context.Context, ref Ref) (path string, ok bool, err error)
}

const stagingDir = "staging"

// HashFile streams path through sha256.
func HashFile(path string) (Ref, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("blob: open staged file: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("blob: hash staged file: %w", err)
	}
	return Ref(hex.EncodeToString(h.Sum(nil))), nil
}

func (s *FileStore) StagingDir() (string, error) {
	dir := filepath.Join(s.root, stagingDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("blob: create staging: %w", err)
	}
	return dir, nil
}

func (s *FileStore) PutFile(ctx context.Context, path string, cache bool) (Ref, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	ref, err := HashFile(path)
	if err != nil {
		return "", err
	}
	class := originalDir
	if cache {
		class = cacheDir
	}
	dst, err := s.path(class, ref)
	if err != nil {
		return "", err
	}
	// The same dedup-and-promote rule as put: a cache write is satisfied by the bytes anywhere, an original only
	// by the bytes in original/.
	if found, ok, err := s.locate(ref); err != nil {
		return "", err
	} else if ok && (cache || found == dst) {
		_ = os.Remove(path)
		return ref, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", fmt.Errorf("blob: create bucket: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("blob: chmod: %w", err)
	}
	// Atomic, and cheap because StagingDir is on this volume. A file staged elsewhere fails here with EXDEV
	// rather than being silently copied: the caller asked for the wrong directory.
	if err := os.Rename(path, dst); err != nil {
		return "", fmt.Errorf("blob: adopt staged file: %w", err)
	}
	return ref, nil
}

func (s *FileStore) Open(ctx context.Context, ref Ref) (io.ReadSeekCloser, error) {
	rc, err := s.Get(ctx, ref)
	if err != nil {
		return nil, err
	}
	// Get returns the *os.File; asserted rather than assumed so a future Get that wraps it fails loudly here.
	f, ok := rc.(*os.File)
	if !ok {
		_ = rc.Close()
		return nil, errors.New("blob: file store returned a non-seekable reader")
	}
	return f, nil
}

func (s *FileStore) LocalPath(ctx context.Context, ref Ref) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	p, ok, err := s.locate(ref)
	if err != nil || !ok {
		if err == nil {
			err = ErrNotFound
		}
		return "", false, err
	}
	return p, true, nil
}

var _ Files = (*FileStore)(nil)

// memoryStaging is one temporary directory per process, created on first use.
var memoryStaging struct {
	once sync.Once
	dir  string
	err  error
}

func (s *MemoryStore) StagingDir() (string, error) {
	memoryStaging.once.Do(func() {
		memoryStaging.dir, memoryStaging.err = os.MkdirTemp("", "blob-staging-")
	})
	return memoryStaging.dir, memoryStaging.err
}

func (s *MemoryStore) PutFile(ctx context.Context, path string, cache bool) (Ref, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("blob: read staged file: %w", err)
	}
	ref, err := s.put(ctx, data, cache)
	if err != nil {
		return "", err
	}
	_ = os.Remove(path)
	return ref, nil
}

type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

func (s *MemoryStore) Open(ctx context.Context, ref Ref) (io.ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	data, ok := s.objects[ref]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	return nopSeekCloser{bytes.NewReader(data)}, nil
}

// LocalPath is never available in memory.
func (s *MemoryStore) LocalPath(context.Context, Ref) (string, bool, error) { return "", false, nil }

var _ Files = (*MemoryStore)(nil)
