package blob

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// FileStore keeps objects on a filesystem, one file per object.
//
// This is the dev implementation and a viable production one on a mounted volume;
// an S3-compatible Store can replace it without touching callers (PRD 008 §11 Q4).
//
// # Layout: the backup scope is a path
//
//	<root>/original/<ab>/<ref>   must be backed up — the only copy of these pixels
//	<root>/cache/<ab>/<ref>      need not be — a rendition, reproducible from an original
//
// Two subtrees rather than a manifest, a naming convention or a database column,
// because of who the consumer is: a backup is an operator (or a cron line) pointing
// restic/rsync at a directory. Anything that requires reading a database, parsing a list
// or matching a filename pattern is something a backup script can get wrong, or can
// silently stop doing when the schema moves. An exclude that is one directory is an
// exclude that stays correct.
//
// The split lives inside one store rather than being two configured stores because the
// two classes must share one dedup domain and one statfs: the disk floor in
// adminstorage.go measures *the volume*, and bytes identical to an existing object
// should not be written twice just because they arrived through a different call.
type FileStore struct {
	root string
}

// The two class subtrees. Names, not two characters, so they can never collide with a
// hash bucket — which is what makes the legacy migration below unambiguous.
const (
	originalDir = "original"
	cacheDir    = "cache"
)

// NewFileStore creates the root directory if needed and returns a store rooted
// there.
//
// It also *enforces* the directory mode rather than only setting it at creation.
// That distinction was found the hard way: when the root is a Docker volume or bind
// mount, the directory already exists before this code runs, so MkdirAll is a
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
	s := &FileStore{root: root}
	if err := s.migrateLegacyLayout(); err != nil {
		return nil, err
	}
	return s, nil
}

// OriginalDir is the directory a backup must cover.
//
// Exported so the thing that needs saying can be said once, in the code, and read by a
// test — rather than living only in a runbook, where it drifts. Task 384's outstanding
// criterion is a backup target confirmed to hold the photograph library; this is the
// path that criterion is about.
func (s *FileStore) OriginalDir() string { return filepath.Join(s.root, originalDir) }

// CacheDir is the directory a backup may skip, and which may be emptied wholesale to
// reclaim space: everything in it can be produced again from an original.
func (s *FileStore) CacheDir() string { return filepath.Join(s.root, cacheDir) }

// migrateLegacyLayout moves objects written before the split into the original subtree.
//
// Before this change every object lived at <root>/<ab>/<ref> with nothing recording
// which kind it was. That information is genuinely gone — a thumbnail and a photograph
// are both just bytes under a hash — so the migration treats **all** of it as original.
// That is the conservative direction: it over-counts the backup by the thumbnails
// already on the volume, where the alternative would drop real photographs out of the
// backup scope on the strength of a guess.
//
// It runs on every boot and is a no-op once nothing is left at the top level, which is
// what makes it safe to leave in place rather than something to remember to remove. A
// bucket moves with a single rename where it can: 256 renames, not one per object.
func (s *FileStore) migrateLegacyLayout() error {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return fmt.Errorf("blob: read root: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() || !isBucketName(e.Name()) {
			// "original", "cache", and anything an operator left lying about. Only a
			// two-hex-character directory can have been written by the old layout.
			continue
		}
		from := filepath.Join(s.root, e.Name())
		to := filepath.Join(s.root, originalDir, e.Name())

		if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
			return fmt.Errorf("blob: create original root: %w", err)
		}
		if err := os.Rename(from, to); err == nil {
			continue
		} else if !errors.Is(err, os.ErrExist) && !errors.Is(err, syscall.ENOTEMPTY) {
			return fmt.Errorf("blob: migrate bucket %s: %w", e.Name(), err)
		}
		// The destination bucket already holds objects — a previous run interrupted
		// partway, or writes landed before this boot. Merge file by file. Identical refs
		// are identical bytes, so an existing destination file is not a conflict to
		// resolve, it is work already done.
		if err := s.mergeLegacyBucket(from, to); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) mergeLegacyBucket(from, to string) error {
	objects, err := os.ReadDir(from)
	if err != nil {
		return fmt.Errorf("blob: read legacy bucket %s: %w", from, err)
	}
	for _, o := range objects {
		if o.IsDir() {
			continue
		}
		src := filepath.Join(from, o.Name())
		dst := filepath.Join(to, o.Name())
		if _, err := os.Stat(dst); err == nil {
			// Same ref, same bytes. Drop the duplicate.
			if err := os.Remove(src); err != nil {
				return fmt.Errorf("blob: remove migrated duplicate %s: %w", src, err)
			}
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("blob: stat %s: %w", dst, err)
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("blob: migrate object %s: %w", src, err)
		}
	}
	// Only succeeds when it is empty, and a failure is not worth failing a boot over: an
	// empty stray directory costs nothing and the next boot tries again.
	_ = os.Remove(from)
	return nil
}

// isBucketName reports whether name is a two-hex-character hash bucket, i.e. something
// the old flat layout could have created.
func isBucketName(name string) bool {
	if len(name) != 2 {
		return false
	}
	_, err := hex.DecodeString(name)
	return err == nil
}

// path maps a Ref to a file path within one class subtree, fanning out on the first two
// hex characters.
//
// The fan-out is not premature optimisation: a flat directory with a few thousand
// portraits is awkward to list and, on some filesystems, slow to look up. Two hex
// characters give 256 buckets, which is plenty at this scale.
//
// It returns an error for an invalid Ref rather than sanitising one, because every
// Ref here should have come from ComputeRef; anything else means a bug or an
// attempt at path traversal, and both deserve to fail loudly.
func (s *FileStore) path(class string, ref Ref) (string, error) {
	if !ref.Valid() {
		return "", fmt.Errorf("blob: invalid ref %q", string(ref))
	}
	name := string(ref)
	return filepath.Join(s.root, class, name[:2], name), nil
}

// locate returns the path of an existing object, searching both classes.
//
// Readers are class-agnostic by design (see the package comment): a Ref on the wire says
// nothing about how the bytes were made, and it must not have to. The cost is one extra
// stat on a cache hit against a local filesystem, which is not worth avoiding by
// teaching every projection a second column.
//
// Original is checked first because it is the larger and more frequently read class —
// full renditions — and because if the same bytes somehow exist in both, the original is
// the copy guaranteed not to be reclaimed.
func (s *FileStore) locate(ref Ref) (string, bool, error) {
	for _, class := range []string{originalDir, cacheDir} {
		p, err := s.path(class, ref)
		if err != nil {
			return "", false, err
		}
		switch _, err := os.Stat(p); {
		case err == nil:
			return p, true, nil
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return "", false, fmt.Errorf("blob: stat: %w", err)
		}
	}
	return "", false, nil
}

func (s *FileStore) Put(ctx context.Context, data []byte) (Ref, error) {
	return s.put(ctx, data, originalDir)
}

func (s *FileStore) PutCache(ctx context.Context, data []byte) (Ref, error) {
	return s.put(ctx, data, cacheDir)
}

// PutAs writes rebuilt bytes under an existing derived name. See the package comment.
func (s *FileStore) PutAs(ctx context.Context, ref Ref, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !ref.Valid() {
		return fmt.Errorf("blob: invalid ref %q", string(ref))
	}

	// The guardrail the whole scheme rests on. If this ref names an original, the bytes
	// under it are the only copy of something and their ref is their checksum; letting a
	// rebuild write here would replace irreplaceable data with a re-encode of it, and
	// would destroy the one property originals are kept verifiable for.
	//
	// Checked rather than assumed, because a repair path holds two refs — a source and a
	// target — and passing them in the wrong order is an ordinary mistake to make.
	orig, err := s.path(originalDir, ref)
	if err != nil {
		return err
	}
	switch _, serr := os.Stat(orig); {
	case serr == nil:
		return fmt.Errorf("blob: refusing to rebuild %s: it names an original, not a rendition", ref)
	case errors.Is(serr, os.ErrNotExist):
		// Good: not an original.
	default:
		return fmt.Errorf("blob: stat: %w", serr)
	}

	return s.writeObject(cacheDir, ref, data)
}

func (s *FileStore) put(ctx context.Context, data []byte, class string) (Ref, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	ref := ComputeRef(data)
	dst, err := s.path(class, ref)
	if err != nil {
		return "", err
	}

	// Already stored: identical contents by definition, so there is nothing to do.
	// This is what makes a projection replay cheap and safe.
	//
	// Deliberately locate rather than a stat of dst, so dedup spans both classes and the
	// same bytes are never written twice. The asymmetry that follows is intended:
	//
	//   - PutCache finding an original is ideal — the bytes are present *and* backed up.
	//   - Put finding only a cache copy would leave the sole copy of an original in the
	//     subtree a backup skips, which is exactly the silent loss this split exists to
	//     prevent. So that case promotes: fall through and write the original too.
	//
	// In practice the second case barely fires, since a full rendition and its thumbnail
	// do not hash alike. It fires for an image small enough that resizing is a no-op —
	// and one such photograph quietly outside the backup is one too many.
	if found, ok, err := s.locate(ref); err != nil {
		return "", err
	} else if ok && (class == cacheDir || found == dst) {
		return ref, nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", fmt.Errorf("blob: create bucket: %w", err)
	}

	if err := s.writeObject(class, ref, data); err != nil {
		return "", err
	}
	return ref, nil
}

// writeObject writes data to the object's path in class, atomically.
//
// Shared by put and PutAs so the durability and permission handling cannot diverge
// between the two — a rebuilt rendition must land exactly as safely as a fresh one.
func (s *FileStore) writeObject(class string, ref Ref, data []byte) error {
	dst, err := s.path(class, ref)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("blob: create bucket: %w", err)
	}

	// Write to a temp file in the same directory, then rename. Rename within a
	// directory is atomic, so a reader can never observe a partially written
	// object — which for a content-addressed store would be worse than a missing
	// one, since the ref would then name bytes that do not hash to it.
	//
	// It matters just as much for a rebuild, for a different reason: the rebuild happens
	// while requests are being served, so a reader must see either the old bytes or the
	// new ones and never a half-written JPEG.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return fmt.Errorf("blob: create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// No-op once the rename has succeeded.
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("blob: write: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("blob: chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("blob: close temp: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("blob: rename: %w", err)
	}
	return nil
}

func (s *FileStore) Get(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, ok, err := s.locate(ref)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		// Raced with a Delete or a cache reclaim between the stat and the open. Same
		// outcome as never having been there, which callers already degrade from.
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
	_, ok, err := s.locate(ref)
	return ok, err
}

// Delete removes the object from both classes.
//
// Both, not "the class it happens to be in": the purge paths (blobpurge.go,
// portraitpurge.go) have already established that nothing references this ref, and
// leaving a copy in the other subtree would mean a deleted photograph still on the disk
// — and still in the backup. Deleting something absent is not an error, so retention
// jobs stay re-runnable.
func (s *FileStore) Delete(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, class := range []string{originalDir, cacheDir} {
		p, err := s.path(class, ref)
		if err != nil {
			return err
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("blob: remove: %w", err)
		}
	}
	return nil
}

var _ Store = (*FileStore)(nil)
