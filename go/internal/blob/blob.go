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
// Content addressing is what makes that safe. A replay re-publishes the same
// reference, and Put with identical bytes is a no-op, so rebuilding projections
// converges without re-uploading anything and without orphaning what is stored.
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

// Store is the storage seam.
//
// Kept thin on purpose: the production choice between object storage and a mounted
// volume is still open (PRD 008 §11 Q4), and a narrow interface is what keeps that
// a wiring change rather than a rewrite. Anything richer — content type, dimensions,
// ownership — belongs in the projection that references the object, not here.
type Store interface {
	// Put stores data and returns its Ref. Idempotent: storing identical bytes
	// twice yields the same Ref and one object.
	Put(ctx context.Context, data []byte) (Ref, error)

	// Get returns a reader for the object, or ErrNotFound. The caller closes it.
	Get(ctx context.Context, ref Ref) (io.ReadCloser, error)

	// Exists reports whether the object is present, without transferring it.
	Exists(ctx context.Context, ref Ref) (bool, error)

	// Delete removes the object. Deleting something absent is not an error, so
	// retention jobs (PRDs 003/007) can be re-run safely.
	Delete(ctx context.Context, ref Ref) error
}
