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

	// SortMode is how the album's photographs will be arranged (PRD 024 §6 R9). The create handler sets
	// `time-asc`; empty means the fold leaves the column at its default, which is `manual`.
	//
	// That is the whole of R9's two-places default, and the asymmetry is deliberate: an album created today
	// is time-ordered because code chose it, while a log replayed from before this field existed folds to
	// `manual` and keeps the arrangement somebody made by hand.
	SortMode string `json:"sortMode,omitempty"`

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

	// CoverPhotoID chooses the photograph the album opens with (task 396). "" clears the choice, which falls
	// back to the first live item — the rule before there was a choice, and still the rule when the chosen one
	// leaves the album or the library.
	CoverPhotoID *string `json:"coverPhotoId,omitempty"`

	// SortMode changes how the album's photographs are arranged. See the constants below for the values.
	//
	// A pointer like the rest, and for the extra reason that the empty string is not a legal mode: nil is the
	// only way to say "not mentioned", so the fold can refuse an empty string as the malformed message it is
	// rather than guessing which mode was meant.
	SortMode *string `json:"sortMode,omitempty"`

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

// The sort modes an album's photographs may be arranged by (PRD 024 §6 R1).
//
// A mode says **when the ordinals are recomputed**, not how a read orders rows: `album_item.ordinal` is what
// the curator's grid and the public page both read, so a mode is applied by rewriting ordinals and
// publishing one reorder (PRD 024 §8 D1). `manual` therefore costs nothing — it means never.
//
// Nothing here concerns `album.sortOrder`, which sequences albums against each other on the frontpage.
//
// Constants rather than free strings for the reason the verdicts above give, with one addition specific to
// these: a mode that is neither `manual` nor a rule anything implements would not error, it would quietly
// stop an album from being re-sorted with nothing to say why. `ValidSortMode` is the one gate, and the fold
// and the handlers both go through it.
const (
	// SortModeManual is a hand arrangement, and the value that means "leave it alone". It is the column's
	// default, so an album written before this existed is never re-sorted behind its curator's back — see
	// table.sql for why that default is not `time-asc`.
	SortModeManual = "manual"
	// SortModeTimeAsc and SortModeTimeDesc order by capture time, falling back to upload time for a
	// photograph whose file did not record one.
	SortModeTimeAsc  = "time-asc"
	SortModeTimeDesc = "time-desc"
	// SortModeFilenameAsc and SortModeFilenameDesc order by filename, compared case-insensitively: two
	// cameras in one album otherwise separate `IMG_*.JPG` from `img_*.jpg` into two blocks.
	SortModeFilenameAsc  = "filename-asc"
	SortModeFilenameDesc = "filename-desc"
)

// SortModes lists the modes in the order a curator is offered them, so that a caller reporting a refusal can
// say what it would have accepted instead of carrying a second copy of the list that drifts from this one.
func SortModes() []string {
	return []string{SortModeManual, SortModeTimeAsc, SortModeTimeDesc, SortModeFilenameAsc, SortModeFilenameDesc}
}

// ValidSortMode reports whether a mode is one this app implements.
//
// The empty string is **not** valid. An absent mode is expressed by not sending the field — on `Created` by
// leaving it empty so the column's default stands, on `Updated` by a nil pointer — and accepting "" here as a
// synonym for `manual` would let a blank select silently un-sort an album.
func ValidSortMode(mode string) bool {
	switch mode {
	case SortModeManual, SortModeTimeAsc, SortModeTimeDesc, SortModeFilenameAsc, SortModeFilenameDesc:
		return true
	}
	return false
}
