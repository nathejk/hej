package photo

import (
	"time"
)

// The photograph event shapes (PRD 022 §8.7, task 363).
//
// # Why they live here
//
// These events are published by **this app**, not by another service, so somebody has to own the shape.
// The convention is the portrait's, the glimt's and the album's: the type is owned by the projection
// that consumes it, and cmd/api imports this package to publish. The alternative — a struct in
// internal/ plus a private copy here, because this package may not import internal/... — is two structs
// that must agree on JSON tags with nothing to catch it when they stop.
//
// # References, never bytes
//
// Every media field is a content hash into the blob store (PRD 008 §8). Putting image bytes on the log
// would make replay ruinous, and content addressing is also what makes the fold idempotent: a replay
// re-publishes the same refs and the projection converges on the same rows.
//
// # One event per fact
//
// Uploaded, updated, location-cleared, patrol-tagged, patrol-untagged and deleted — rather than one
// "changed" event with nullable fields. Same reasoning as glimt's and album's: a replay must be able to
// tell a deliberate removal from a malformed message, and the log should record *why* a photograph or a
// coordinate disappeared, which is the question anyone auditing a takedown will actually ask.

// Uploaded puts one photograph in the year's library.
//
// # The id is derived, not minted
//
// PhotoID is the content hash of the stored full rendition, which makes this event — and therefore the
// whole upload path — idempotent by construction (PRD 022 §8.5). A photographer re-dragging a folder
// republishes this event with the same id and the fold converges on the same row rather than growing a
// duplicate.
//
// The fold must **not** clear `deleted` when it upserts, and that rule is stronger here than in album's
// create fold. Because the id is content-derived, clearing it would mean a re-upload silently reverses a
// deletion — an objection honoured on Tuesday undone on Wednesday by somebody who was never told there
// was one. See table.sql's note on `deleted`.
//
// # Unpublished by construction
//
// There is no `published` field, and not because it defaults to false: a photograph in the library is
// not publishable at all. Visibility is a property of the **album** that references it, so nothing here
// can put a photograph on the open web. That is the safety property PRD 011 §0b relies on, expressed as
// an absence rather than a flag somebody has to remember to leave alone.
type Uploaded struct {
	PhotoID string `json:"photoId"`
	Year    string `json:"year"`

	// Ref and ThumbRef are content hashes. ThumbRef may be "" when one could not be produced; readers
	// fall back to the full image rather than rendering a gap, as the glimt grid does.
	Ref      string `json:"ref"`
	ThumbRef string `json:"thumbRef,omitempty"`

	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	Bytes  int `json:"bytes,omitempty"`

	// Location is where the camera said the photograph was taken, read from EXIF **before** the bytes
	// were re-encoded (which strips it), together with the verdict the race-area check reached.
	//
	// Nil when the file carried no usable fix, which is the common case. A struct pointer rather than
	// three optional fields, for the reason Updated's doc gives at length: a coordinate and the
	// judgement made about it are one fact.
	Location *Location `json:"location,omitempty"`

	UploadedAt time.Time `json:"uploadedAt"`
}

// Location is a coordinate together with what the race-area check made of it.
//
// # Why these three fields travel as one value
//
// Because a coordinate without its verdict is dangerous and a verdict without its coordinate is a lie,
// and keeping them as separate optional fields on an event makes both expressible. The map read filters
// on the verdict, so a message that changed the coordinate and forgot the verdict would leave a
// photograph plotted at its *old* judgement — a point somebody moved out of the race area still on the
// public map, because the field that governs plotting was not mentioned.
//
// One value makes that unsayable: you cannot set a position without stating what was decided about it.
// The verdict is therefore not optional here, and the fold re-checks it against the known constants
// rather than trusting the publisher (see consumer.go).
//
// Lat and Lng are plain float64 rather than pointers because within a Location they are not optional —
// the *Location itself* is what is optional. 0,0 is a real place in the Atlantic and also what a camera
// with no fix writes, which is why `imaging.ReadGPS` refuses it and why "no coordinate" is expressed by
// a nil Location rather than by zeroes.
type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`

	// BoundsVerdict is BoundsNone, BoundsInside, BoundsOutside or BoundsUnknown.
	//
	// Decided at the moment the coordinate was set and carried on the event rather than recomputed at
	// read time, because the race area changes as organizers site checkpoints — so a verdict recomputed
	// next week would be a different verdict, silently, for a photograph nobody touched. The event
	// records what was decided when the coordinate was accepted, which is the fact a curator was shown.
	BoundsVerdict string `json:"boundsVerdict"`
}

// Updated changes a photograph's editorial fields.
//
// # Why every field is a pointer
//
// So that "not mentioned" and "set to empty" are different messages. A curator clearing a caption and a
// curator placing a location must not be the same event, or one of the two silently wipes the other's
// field. This is the delta shape `album.Updated` uses and shared-go's vehicle `UpdateFields` before it,
// and it exists for the same reason: the fold applies exactly what was sent.
//
// It matters more here than in either of those, because this is the event a **bulk** action publishes.
// Setting a location on forty selected photographs must not also blank forty captions the curator spent
// an evening writing, and with a non-pointer shape that is one forgotten field away.
//
// # Clearing a location is not expressible here
//
// Deliberately. `Location: nil` means "not mentioned", so there would be no way to say "remove the
// coordinate" — and overloading nil to mean removal is what makes the caption bug above possible for
// positions too. Clearing is its own event, LocationCleared, which also means the log records the
// intent rather than an absence. See PRD 022 §8.7.
type Updated struct {
	PhotoID string `json:"photoId"`
	Year    string `json:"year"`

	Caption *string `json:"caption,omitempty"`

	// Location, when present, replaces the coordinate and its verdict together.
	Location *Location `json:"location,omitempty"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// LocationCleared removes a photograph's coordinate.
//
// Its own event rather than an Updated carrying an empty value, for the reason the package doc gives:
// the log should record that somebody *decided* the coordinate was wrong. A curator clearing a bad fix
// and a photograph that never had one end up in the same state — no pin — and that is correct for every
// read, but they are not the same act and only one of them is a judgement worth keeping.
//
// The fold sets the verdict to BoundsNone along with the NULLs, because a verdict about a coordinate
// that no longer exists would claim a judgement about nothing.
type LocationCleared struct {
	PhotoID string `json:"photoId"`
	Year    string `json:"year"`

	// Reason is the curator's note, optional and never shown publicly.
	Reason string `json:"reason,omitempty"`

	ClearedAt time.Time `json:"clearedAt"`
}

// PatrolTagged attributes a photograph to a patrol (task 367).
//
// # Both the id and the number, and this is not redundancy
//
// `teamNumber` is **not unique per year** — `public_patrol`'s index is deliberately non-unique and its
// only read does `ORDER BY teamId LIMIT 1`. A tag that stored only the number would therefore be free
// to start pointing at a different patrol, and a tag that stored only the id would be unreadable to a
// curator, who knows patrols by the number on the sign.
//
// So the resolved TeamID is the identity and the Number is what it was resolved *from*, captured at the
// moment a curator confirmed the name. PRD 022 §8.6.
//
// # A patrol, never a person
//
// There is no field here for a member, and none may be added. A photograph is attributed to a patrulje,
// which is the rule the public patrol page and the public glimt page already work under (PRD 011 §4,
// `.rules`). A tagging surface is precisely where somebody would reach for a name, which is why the
// absence is stated rather than left to be noticed.
//
// # In v1 a tag has no public effect
//
// PRD 022 §11 Q1: tagging is curator metadata for now, and surfacing it on the patrol's public page is
// agreed as the direction but deferred to its own task and its own review. **No public read may be
// written against this until that decision is taken** — a mistag must be correctable before it can put
// a photograph on the wrong family's page.
type PatrolTagged struct {
	PhotoID string `json:"photoId"`
	Year    string `json:"year"`

	// TeamID is the resolved patrol, and the identity of the tag.
	TeamID string `json:"teamId"`
	// Number is the patrol number the curator typed, kept for display and for the record.
	Number string `json:"number,omitempty"`

	TaggedAt time.Time `json:"taggedAt"`
}

// PatrolUntagged removes an attribution.
//
// A separate verb rather than a tombstone on PatrolTagged, matching album's item-removed: a replay must
// be able to tell a deliberate untag from a malformed tag.
type PatrolUntagged struct {
	PhotoID string `json:"photoId"`
	Year    string `json:"year"`

	TeamID string `json:"teamId"`

	UntaggedAt time.Time `json:"untaggedAt"`
}

// Deleted takes a photograph out of the library, and therefore out of every album.
//
// A soft delete. The reason is album's — a removal may honour somebody's objection and an accidental one
// must be recoverable — plus the content-addressing consequence recorded on Uploaded and in table.sql.
//
// Note what this is *not*: removing a photograph from one album is `album.ItemRemoved`, and it leaves
// the photograph in the library and in every other album. The two are different acts and PRD 022 §5
// requires the difference to be obvious in the curator's copy, because one of them is what an organizer
// means when they say "take it down".
type Deleted struct {
	PhotoID string `json:"photoId"`
	Year    string `json:"year"`

	// Reason is the curator's note. Optional, and never shown publicly.
	Reason string `json:"reason,omitempty"`

	DeletedAt time.Time `json:"deletedAt"`
}

// The bounds verdicts. Four values, and the reasoning for each is in table.sql.
//
// Constants rather than free strings because the map read filters on this column: a typo'd verdict would
// not error, it would quietly make a photograph unplottable — or, worse, plot one that had been judged
// out of bounds.
//
// # Why these are duplicated from the album package
//
// They are the same four strings `album` declares, and the values must stay identical because the album
// fold still has to read them off legacy `itemadded` events (task 364, PRD 022 §8.7).
//
// The duplication is deliberate all the same: **this is now the canonical home**, because after PRD 022
// §8.3 a verdict is a fact about a photograph and no longer about an album position. `album`'s copies
// are legacy, kept only so the fold can tolerate events published before the library existed. A shared
// constant would have been the obvious move and is the wrong one — it would outlive the legacy fold and
// leave a permanent dependency between two projections to express agreement that is only temporarily
// needed.
const (
	// BoundsNone means there is no coordinate. Nothing to plot, nothing wrong.
	BoundsNone = "none"
	// BoundsInside means the coordinate is in the race area, so it may be plotted.
	BoundsInside = "inside"
	// BoundsOutside means the coordinate is not in the race area. Kept, never plotted.
	BoundsOutside = "outside"
	// BoundsUnknown means there was a coordinate but no race area to judge it against.
	//
	// A statement about us, not about the photograph — which is why it is not folded into BoundsOutside.
	// Conflating them would condemn every upload made before the checkpoints were sited, permanently,
	// with no way to tell those apart from genuinely stray coordinates.
	BoundsUnknown = "unknown"
)

// Plottable reports whether a verdict allows a photograph on the map.
//
// One function so the rule lives in one place: only `inside` is plottable. `unknown` deliberately is not
// — we could not check it, and plotting an unchecked coordinate on a public page is the failure this
// whole verdict exists to prevent.
func Plottable(verdict string) bool { return verdict == BoundsInside }
