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
type ItemAdded struct {
	AlbumID string `json:"albumId"`
	Year    string `json:"year"`

	// Ordinal is the curator's position for this item. Explicit, because the ordering is editorial.
	Ordinal int `json:"ordinal"`

	// Ref and ThumbRef are content hashes. ThumbRef may be "" when one could not be produced; the
	// page falls back to the full item rather than rendering a gap, as the glimt grid does.
	Ref      string `json:"ref"`
	ThumbRef string `json:"thumbRef,omitempty"`

	Caption string `json:"caption,omitempty"`

	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	Bytes  int `json:"bytes,omitempty"`

	// Lat and Lng are where the photograph was taken, read from EXIF **before** the bytes were
	// re-encoded (which strips it). Nil when the file carried no usable fix, which is the common case.
	//
	// Pointers rather than zero values because 0,0 is a real place in the Atlantic and also what a
	// camera with no fix writes — the same reasoning the checkpoint projection applies to an unset
	// position. `imaging.ReadGPS` already refuses 0,0, and this shape means the refusal survives
	// serialisation instead of arriving as a coordinate off Ghana.
	Lat *float64 `json:"lat,omitempty"`
	Lng *float64 `json:"lng,omitempty"`

	// BoundsVerdict is what the race-area check made of that coordinate: BoundsNone, BoundsInside,
	// BoundsOutside or BoundsUnknown.
	//
	// Decided at ingest and carried on the event rather than recomputed at read time, because the race
	// area changes as organizers site checkpoints — so a verdict recomputed next week would be a
	// different verdict, silently, for a photograph nobody touched. The event records what was
	// decided when the photograph was accepted, which is the fact a curator was shown.
	BoundsVerdict string `json:"boundsVerdict"`

	AddedAt time.Time `json:"addedAt"`
}

// The bounds verdicts. Four values, and the reasoning for each is in table.sql.
//
// Constants rather than free strings because the projection filters the map read on this column: a
// typo'd verdict would not error, it would quietly make a photograph unplottable — or, worse, plot one
// that had been judged out of bounds.
const (
	// BoundsNone means the photograph carried no usable coordinate. Nothing to plot, nothing wrong.
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
