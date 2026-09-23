package album

import (
	"time"
)

// The album event shapes (PRD 011 §6, task 333).
//
// # Why they live here
//
// These events are published by **this app**, not by another service, so somebody has to own the
// shape. The convention is the portrait's and the glimt's (see glimt/events.go): the type is owned by
// the projection that consumes it, and cmd/api imports this package to publish. The alternative — a
// struct in internal/ plus a private copy here, because this package may not import internal/... — is
// two structs that must agree on JSON tags with nothing to catch it when they stop.
//
// # References, never bytes
//
// Every media field is a content hash into the blob store (PRD 008 §8). Putting image bytes on the log
// would make replay ruinous, and content addressing is also what makes the fold idempotent: a replay
// re-publishes the same refs and the projection converges on the same rows.
//
// # One event per fact
//
// Created, updated, item-added, and (task 335) item-removed and deleted — rather than one "changed"
// event with nullable fields. Same reasoning as glimt's: a replay must be able to tell a deliberate
// removal from a malformed message, and the log should record *why* a photograph disappeared, which is
// the question anyone auditing a takedown will actually ask.

// Created opens an album.
//
// Unpublished by default is deliberate and is expressed by the absence of `Published` here rather than
// by a false field: an album is assembled over several sittings, and a create event that could publish
// would mean the first item is on the open web before the second is chosen. Publishing is an `Updated`.
type Created struct {
	AlbumID string `json:"albumId"`
	Year    string `json:"year"`

	// Slug is the URL segment. Frozen at creation: retitling must not break a link already shared.
	Slug string `json:"slug"`

	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sortOrder,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// Updated changes an album's editorial fields, including whether it is published.
//
// # Why every field is a pointer
//
// So that "not mentioned" and "set to empty" are different messages. A curator clearing a description
// and a curator renaming an album must not be the same event, or one of the two silently wipes the
// other's field. This is the same delta shape shared-go's vehicle `UpdateFields` uses, and it exists
// for the same reason: the fold applies exactly what was sent.
//
// The slug is **not** updatable. It is the address of a page somebody may already have in a family
// chat, and an album that answers 404 because it was retitled is worse than one whose URL no longer
// matches its name.
type Updated struct {
	AlbumID string `json:"albumId"`
	Year    string `json:"year"`

	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	SortOrder   *int    `json:"sortOrder,omitempty"`
	Published   *bool   `json:"published,omitempty"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// ItemAdded puts one photograph in an album.
//
// # This event used to carry the photograph
//
// Before PRD 022 it named the bytes directly: a `ref`, a thumbnail, a caption, dimensions and a
// coordinate. It now names a **photograph in the library** and nothing else, because that is all an
// album membership is (PRD 022 §8.3).
//
// This is a breaking change to a published event shape, and the only honest justification is that **no
// album has ever been created outside the development fixture** — there was no curation tool, which is
// why PRD 022 exists. There is no production history to respect here, only dev history, and the fold
// tolerates the old shape rather than erroring on it (see handleItemAdded).
//
// # What the fold gains from the narrowing
//
// Nothing to validate but an id and an ordinal. The refs, the dimensions and the verdict are `photo`'s
// problem and are validated once where they are written, rather than on every event that mentions a
// photograph — which also means a photograph in three albums can no longer arrive with three different
// captions.
type ItemAdded struct {
	AlbumID string `json:"albumId"`
	Year    string `json:"year"`

	// Ordinal is the curator's position for this item. Explicit, because the ordering is editorial.
	Ordinal int `json:"ordinal"`

	// PhotoID is the library photograph this position holds — a content hash, validated as one.
	//
	// Its absence is how the fold recognises a legacy event and skips it, so this field is load-bearing
	// beyond simply naming the photograph.
	PhotoID string `json:"photoId"`

	AddedAt time.Time `json:"addedAt"`
}

// The bounds verdicts, kept here for the readers that still name them.
//
// **`photo` is the canonical home for these** (PRD 022 §8.3): after the library split, a verdict is a
// fact about a photograph rather than about an album position, and `photo.Bounds*` is what new code should
// use. These stay because the album *querier* still selects the column — through a join to `photo` — and
// because `cmd/api` has callers that were written against them.
//
// The values must stay byte-identical to `photo`'s. They are duplicated rather than aliased on purpose:
// an alias would make the two packages permanently dependent to express an agreement that is only needed
// while both names exist, and the compiler cannot check a string constant's meaning either way. The guard
// is `TestVerdictsAgreeWithThePhotoPackage`, which fails if they drift.
//
// Constants rather than free strings because the map read filters on this column: a typo'd verdict would
// not error, it would quietly make a photograph unplottable — or, worse, plot one that had been judged
// out of bounds.
const (
	// BoundsNone means there is no usable coordinate. Nothing to plot, nothing wrong.
	BoundsNone = "none"
	// BoundsInside means the coordinate is in the race area, so it may be plotted.
	BoundsInside = "inside"
	// BoundsOutside means the coordinate is not in the race area. Kept, never plotted.
	BoundsOutside = "outside"
	// BoundsUnknown means there was a coordinate but no race area to judge it against.
	//
	// A statement about us, not about the photograph — which is why it is not folded into
	// BoundsOutside. Conflating them would condemn every upload made before the checkpoints were
	// sited, permanently, with no way to tell those apart from genuinely stray coordinates.
	BoundsUnknown = "unknown"
)

// Plottable reports whether a verdict allows a photograph on the map.
//
// One function so the rule lives in one place: only `inside` is plottable. `unknown` deliberately is
// not — we could not check it, and plotting an unchecked coordinate on a public page is the failure
// this whole verdict exists to prevent.
func Plottable(verdict string) bool { return verdict == BoundsInside }
