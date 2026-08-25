package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileStore keeps objects on a filesystem, one file per object.
//
// This is the dev implementation and a viable production one on a mounted volume;
// an S3-compatible Store can replace it without touching callers (PRD 008 §11 Q4).
type FileStore struct {
	root string
}

// NewFileStore creates the root directory if needed and returns a store rooted
// there.
//
// It also *enforces* the directory mode rather than only setting it at creation.
// That distinction was found the hard way: when the root is a Docker volume or bind
// mount, the directory already exists before this code runs, so `MkdirAll` is a
// no-op and leaves whatever mode the container runtime chose — in practice 0755,
// world-readable. Portraits of identifiable minors were therefore readable by any
// process or user on the host that could reach the volume, despite the 0600 on each
// file.
func NewFileStore(root string) (*FileStore, error) {
	if root == "" {
		return nil, errors.New("blob: empty root directory")
	}
	// 0o700 rather than 0o755: these are photographs of identifiable minors
	// (PRDs 003/007). Nothing but this service has any business reading them, and a
	// world-readable directory on a shared volume is the kind of default that is
	// never noticed until it matters.
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("blob: create root: %w", err)
	}
	// Failing here rather than warning is deliberate: if the directory cannot be made
	// private, the right outcome is not to store minors' photographs in it. main
	// treats this as "blob store unavailable" and falls back to memory, which loses
	// persistence but does not quietly expose anything.
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, fmt.Errorf("blob: secure root %s: %w", root, err)
	}
	return &FileStore{root: root}, nil
}

// path maps a Ref to a file path, fanning out on the first two hex characters.
//
// The fan-out is not premature optimisation: a flat directory with a few thousand
// portraits is awkward to list and, on some filesystems, slow to look up. Two hex
// characters give 256 buckets, which is plenty at this scale.
//
// It returns an error for an invalid Ref rather than sanitising one, because every
// Ref here should have come from ComputeRef; anything else means a bug or an
// attempt at path traversal, and both deserve to fail loudly.
func (s *FileStore) path(ref Ref) (string, error) {
	if !ref.Valid() {
		return "", fmt.Errorf("blob: invalid ref %q", string(ref))
	}
	name := string(ref)
	return filepath.Join(s.root, name[:2], name), nil
}

func (s *FileStore) Put(ctx context.Context, data []byte) (Ref, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	ref := ComputeRef(data)
	dst, err := s.path(ref)
	if err != nil {
		return "", err
	}

	// Already stored: identical contents by definition, so there is nothing to do.
	// This is what makes a projection replay cheap and safe.
	if _, err := os.Stat(dst); err == nil {
		return ref, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("blob: stat: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", fmt.Errorf("blob: create bucket: %w", err)
	}

	// Write to a temp file in the same directory, then rename. Rename within a
	// directory is atomic, so a reader can never observe a partially written
	// object — which for a content-addressed store would be worse than a missing
	// one, since the ref would then name bytes that do not hash to it.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return "", fmt.Errorf("blob: create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// No-op once the rename has succeeded.
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("blob: write: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("blob: chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("blob: close temp: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return "", fmt.Errorf("blob: rename: %w", err)
	}
	return ref, nil
}

func (s *FileStore) Get(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, err := s.path(ref)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("blob: open: %w", err)
	}
	return f, nil
}

func (s *FileStore) Exists(ctx context.Context, ref Ref) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	p, err := s.path(ref)
	if err != nil {
		return false, err
	}
	switch _, err := os.Stat(p); {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("blob: stat: %w", err)
	}
}

func (s *FileStore) Delete(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := s.path(ref)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("blob: remove: %w", err)
	}
	return nil
}

var _ Store = (*FileStore)(nil)
