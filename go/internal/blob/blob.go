// Package blob stores opaque binary objects addressed by the hash of their
// contents.
//
// It exists for one thing today: portrait bytes (PRD 003/007). Those are the only
// data in this service that cannot be rebuilt by replaying the event log — the
// event carries a reference, not the image — which makes this package the whole
// backup scope (PRD 008 §8, task 063) and the reason it is kept clear of the
// projection tables. A projection rebuild truncates and refills its tables; it must
// never be able to do that to a portrait.
//
// (It has since grown glimt media and the photograph library. "The whole backup scope"
// still holds for the package; which *part* of the package is refined below.)
//
// Content addressing is what makes that safe. A replay re-publishes the same
// reference, and Put with identical bytes is a no-op, so rebuilding projections
// converges without re-uploading anything and without orphaning what is stored.
//
// # Two classes of object, because only one of them is irreplaceable
//
// "The whole backup scope" was true while the store held portraits. It is no longer
// the useful statement, because most of what is now in here is **derived**: a
// thumbnail is a resize of bytes the store already holds, produced at upload by
// `internal/imaging`. It cannot be replayed from the log, but it does not need to be —
// it can be computed again from its source.
//
// So an object is stored in one of two classes:
//
//   - **original** (Put) — the only copy of these pixels. Losing it loses the
//     photograph. Must be backed up.
//   - **cache** (PutCache) — a rendition derived from an original. Losing it costs
//     some bandwidth and a regeneration. Need not be backed up.
//
// This matters at a scale the portrait era did not have. PRD 022 §6 puts one event's
// hand-in at order 1 GB and §11 Q2 resolved that those photographs are **never
// purged**, so the store grows monotonically, one event per year, forever — and task
// 384's outstanding criterion is a backup target confirmed to hold that trajectory,
// with a restore spot-checked. Every derived byte in the backup makes that job larger
// and a restore slower for no recoverable value.
//
// The distinction is deliberately expressed as **on-disk layout** rather than as a
// column somewhere. A backup is an operator running `restic` or `rsync` over a volume;
// it has no database connection and should not need one. `FileStore` therefore keeps
// the two classes in separate subtrees, so the backup scope is a **path** — see
// `FileStore.OriginalDir`.
//
// Nothing else changes: refs are still content hashes, still class-agnostic on the
// wire, and Get/Exists/Delete still take a bare Ref. A caller reading bytes has no
// business knowing whether they were cheap to make. Only the writer, which knows where
// the bytes came from, picks a class.
//
// # A derived ref is a name; an original ref is a hash
//
// One consequence of the split is worth stating on its own, because it is what makes a
// lost rendition rebuildable (task 430).
//
// Rebuilding a thumbnail does not reproduce its bytes: a different Go release, libjpeg
// build or quality setting re-encodes the same pixels slightly differently, so the hash
// changes. Since the projections hold the old ref, a rebuild that insisted on content
// addressing would have to rewrite rows — from a read path, via the event stream, because
// PRD 008 §8 says the row is written by the consumer and never the writer.
//
// So for derived objects the ref is treated as a **stable name** rather than a checksum:
// `PutAs` writes rebuilt bytes under the ref that was already recorded. Nothing outside
// this package has to change, because nothing outside this package was relying on a
// thumbnail's ref being verifiable — only on it being the address of that thumbnail.
//
// This is confined to the cache class, and that boundary is the whole safety argument:
//
//   - An **original**'s ref remains exactly its content hash, always. That is the data
//     that cannot be rebuilt, so it keeps the property that lets it be verified, and
//     `PutAs` refuses outright if the ref names one.
//   - A **derived** object loses that property, and loses nothing by losing it: an object
//     that cannot be verified is fine when it can be discarded and made again, which is
//     the definition of this class. The class that is unverifiable is exactly the class
//     that is expendable.
//
// The visible cost is caching. The media routes serve `ETag: "<ref>"` with `immutable`,
// so a client holding the pre-rebuild bytes will not revalidate. That is accepted rather
// than overlooked: a rebuild runs the same source through the same recipe, so the stale
// copy is a perceptually identical thumbnail, differing in JPEG entropy coding. Changing
// what an image *is* still requires a new ref, because that is a new upload.
package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

// ErrNotFound is returned by Get when no object has the given ref.
//
// Callers are expected to degrade rather than fail: a portrait whose bytes have
// gone missing must render as "no photo", never as a broken page (PRD 008 §8).
var ErrNotFound = errors.New("blob not found")

// Ref identifies an object by the hash of its contents.
//
// Deliberately a distinct type rather than a string: a Ref travels through event
// bodies and database rows, and being able to confuse it with a filename, a user id
// or a URL path is how a content-addressed store stops being content-addressed.
type Ref string

// String returns the canonical textual form, which is what goes on the wire and
// into projections.
func (r Ref) String() string { return string(r) }

// Valid reports whether r looks like a hash this package produced.
//
// Storage implementations must check this before touching the filesystem: a Ref
// arrives from an event body or a URL, so it is untrusted input, and "../../etc"
// is a Ref-shaped string.
func (r Ref) Valid() bool {
	if len(r) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(string(r))
	return err == nil
}

// ComputeRef returns the Ref for the given bytes.
func ComputeRef(data []byte) Ref {
	sum := sha256.Sum256(data)
	return Ref(hex.EncodeToString(sum[:]))
}

// FreeSpacer is a store that can report the free space of the volume it lives on.
//
// # An optional capability, not a widening of Store
//
// `Store` is deliberately thin because the choice between object storage and a mounted volume is still open
// (PRD 008 §11 Q4). "How much room is left" has no answer for a memory store and a different one for a
// bucket, so it is a separate interface a store may satisfy: callers type-assert and **fail open** when it
// is absent. A memory store has no volume, and refusing uploads in a test because of that would be
// nonsense.
//
// Declared here rather than beside the only implementation because the implementation is platform-specific
// (`free_unix.go`) and callers are not — an interface that vanished on a platform would take `cmd/api` with
// it.
type FreeSpacer interface {
	// FreeBytes returns the bytes available to this process on the volume holding the objects.
	FreeBytes() (uint64, error)
}

// Store is the storage seam.
//
// Kept thin on purpose: the production choice between object storage and a mounted
// volume is still open (PRD 008 §11 Q4), and a narrow interface is what keeps that
// a wiring change rather than a rewrite. Anything richer — content type, dimensions,
// ownership — belongs in the projection that references the object, not here.
type Store interface {
	// Put stores data as an **original** and returns its Ref. Idempotent: storing
	// identical bytes twice yields the same Ref and one object.
	//
	// Original is the default — i.e. plain Put means "back this up" — because the two
	// mistakes are not symmetric. Classifying a derived rendition as an original
	// wastes backup space, which an operator can see and which costs nobody a
	// photograph. Classifying the only copy of some pixels as cache silently drops it
	// from the backup, and that is discovered during a restore, which is the worst
	// possible moment to discover anything.
	Put(ctx context.Context, data []byte) (Ref, error)

	// PutCache stores data as a **derived** object: a rendition that can be produced
	// again from an original this store also holds. Otherwise identical to Put.
	//
	// Only call this when the statement above is actually true. The test is not "is it
	// small" or "is it a thumbnail", it is: *if these bytes vanished, could the service
	// rebuild them from something it still has?* If the source was never stored — as
	// for a glimt, which keeps no original (see glimtmedia.go) — then the rendition is
	// the original, however resized it was on the way in, and it goes through Put.
	PutCache(ctx context.Context, data []byte) (Ref, error)

	// PutAs stores data as a derived object under a ref the **caller** chooses, rather
	// than under the hash of the bytes.
	//
	// This is for rebuilding a rendition that has been lost: see "A derived ref is a
	// name" in the package comment for why it is sound, and why it is confined to the
	// cache class. It refuses a ref that already names an original, so it can never be
	// used to overwrite irreplaceable bytes — the one thing that would make the split
	// above meaningless.
	//
	// It returns no Ref, deliberately: the caller supplied it, and handing it back would
	// suggest it had been derived from the bytes when the entire point is that it was not.
	PutAs(ctx context.Context, ref Ref, data []byte) error

	// Get returns a reader for the object, or ErrNotFound. The caller closes it.
	Get(ctx context.Context, ref Ref) (io.ReadCloser, error)

	// Exists reports whether the object is present, without transferring it.
	Exists(ctx context.Context, ref Ref) (bool, error)

	// Delete removes the object. Deleting something absent is not an error, so
	// retention jobs (PRDs 003/007) can be re-run safely.
	Delete(ctx context.Context, ref Ref) error
}
