// Package kort projects the printed map sheets and map sets that HQ defines, so this app can
// tell a patrol which sheets it has been handed and which checkpoints those sheets reveal
// (PRD 016).
//
// # Why the message types are mirrored here rather than imported
//
// These events are owned by the `hq` repo, where the Go types live in
// `go/nathejk/table/kort/messages.go`. They have **not** been lifted to `nathejk/shared-go`,
// and deliberately so: a projection is lifted once it has stabilised, and part of stabilising
// is having more than one consumer — a single consumer does not exercise all the variants, so
// shapes lifted early get lifted wrong.
//
// So we decode the JSON ourselves, against the types below. This is what the contract
// anticipates — "another service can consume these events today by decoding the JSON itself".
// We are that second consumer, and what we find awkward is what tells HQ which parts of the
// shape are real.
//
// **Nothing in this repo may import hq.** No Go import, no shared module, no path to another
// checkout, no HTTP call. The only cross-repo dependency is the event shapes on the stream, and
// those are pinned in this repo's own copy of the contract:
//
//	roadmap/api/kort-events.md   (copied from hq 2026-09-15)
//
// That copy is the source of truth for the shapes below, and HQ's PRD 010 is where a change to
// them has to be agreed first. HQ's task 138 tracks the eventual lift to shared-go, at which
// point this file becomes a deletion.
//
// # Decoding is tolerant of unknown fields, on purpose
//
// These shapes are unstable by their owner's own assessment, and an additive upstream field —
// exactly how `mapId` reached `qr.registered` — must not break a consumer in the middle of an
// event. So encoding/json's default behaviour is what we want, and `DisallowUnknownFields` here
// would look like rigour and behave like an outage. What we do *not* ignore is a body we cannot
// make sense of at all: the consumer logs those, because they are the signal that our copy of
// the contract has gone stale.
//
// # The copy will go stale, and that is planned for
//
// It already has once: the copy's "What is not published yet" section claims the QR-to-sheet
// link does not exist, when in fact skan publishes it and HQ's own patrol page renders it.
// Designing against the prose alone would have produced a worse feature. So **the code is the
// contract and the document is a guide to it**; the copy is refreshed at the start of each
// season, and on any decode we cannot make sense of (PRD 016 §11.11).
//
// # Where it lives
//
// Alongside `checkpoint` and `person` under nathejk/table/, so it takes only the cqrs
// interfaces and must never import nathejk.dk/internal/... — see the person package's doc for
// the full reasoning.
package kort

import (
	"github.com/nathejk/shared-go/types"
)

// KortID identifies one printed sheet, KortsaetID one set of sheets.
//
// Local types rather than aliases of a shared one, because these entities exist only in HQ's
// model today. When the lift happens they become the shared ids, and this is the one line that
// changes.
type (
	KortID     string
	KortsaetID string
)

// Format is what was printed, and on what.
//
// A closed set rather than free text: it drives no logic, but it is read by humans deciding what
// to hand over, and four values that mean something beat a column holding "A4", "a4" and
// "A4 (dobbeltsidet)".
type Format string

const (
	FormatA4 Format = "a4"
	FormatA3 Format = "a3"

	// FormatSkitse is a hand-drawn slip showing the next group of checkpoints.
	//
	// The awkward case, and the reason this package matters more than it looks. A skitse has
	// **no QR code**, so it is never scanned, so no qr.registered event can ever name it — it
	// cannot appear in the handout projection at all. It is handed over at a post, and its
	// checkpoints are revealed off that post instead (see HandoutCheckgroupID). Its
	// CheckpointIDs are its only trace in the system, and it usually has no extent.
	FormatSkitse Format = "skitse"

	FormatAndet Format = "andet"
)

// Valid reports whether f is one of the known formats.
//
// Used to say so in a log line, not to gate behaviour: an unknown format is stored as-is by the
// projection, because a consumer must not lose a sheet just because a fifth format was added
// upstream.
func (f Format) Valid() bool {
	switch f {
	case FormatA4, FormatA3, FormatSkitse, FormatAndet:
		return true
	}
	return false
}

// Extent is one rectangle of ground a sheet shows.
//
// Corners are north-west and south-east rather than "first" and "second": they are normalised
// before publishing, so every reader can assume the pair is well-formed and nobody has to do a
// min/max dance before handing them to a map library.
//
// A sheet carries zero, one or two of these. Zero is normal (a skitse, or a sheet described
// before anyone drew it); two is a double-sided sheet, which is **one** sheet with two areas —
// one QR, one handover, one reveal. Nothing says which side is the front, and the checkpoints
// are not split per side, so do not try to infer sides.
type Extent struct {
	NorthWest types.Position `json:"northWest"`
	SouthEast types.Position `json:"southEast"`
}

// Created records a new sheet, on NATHEJK.{year}.kort.{kortId}.created.
//
// Only the set and the name. A sheet is *described* after it exists — an operator adds "Kort 3"
// before knowing its format or drawing its area — so a consumer must not wait for a complete
// sheet, and must tolerate a KortsaetID naming a set it has not seen yet: events arrive in
// stream order and a sheet may legitimately precede its set.
type Created struct {
	KortID     KortID     `json:"kortId"`
	KortsaetID KortsaetID `json:"kortsaetId"`
	Name       string     `json:"name"`
}

// Updated records a change to one or more of a sheet's fields — a **patch, not a snapshot**.
//
// Every field is a pointer, and **an absent field means "unchanged"**, not "empty". This is the
// single thing most likely to be got wrong here, and the failure is silent: treating the body as
// a whole record blanks the fields it does not mention, so an event that only renames a sheet
// would erase the checkpoint list that decides what a patrol may see. The checkpoint picker and
// the sheet's description are separate screens that save separately, so partial updates are the
// normal traffic rather than an edge case.
//
// CheckpointIDs and Extents are pointers **to slices** for the same reason, and the distinction
// carries more weight here than usual: nil means untouched, while a pointer to an empty slice
// means "this sheet now has no checkpoints" — a real edit that a plain nil slice could not
// express.
type Updated struct {
	KortID        KortID                `json:"kortId"`
	KortsaetID    *KortsaetID           `json:"kortsaetId,omitempty"`
	Name          *string               `json:"name,omitempty"`
	Format        *Format               `json:"format,omitempty"`
	Note          *string               `json:"note,omitempty"`
	SortOrder     *int                  `json:"sortOrder,omitempty"`
	CheckpointIDs *[]types.CheckpointID `json:"checkpointIds,omitempty"`
	Extents       *[]Extent             `json:"extents,omitempty"`

	// HandoutCheckgroupID changes where the sheet is handed out, and therefore when its
	// checkpoints become visible to the patrol. It is the sheet's *reveal trigger*:
	//
	//   - a checkgroup id → the sheet is handed out at that group's post, so its checkpoints
	//     become visible once the patrol has reached that checkgroup. This is how a skitse
	//     works.
	//   - "" → the QR rule: the checkpoints become visible when the sheet's own QR code is
	//     bound to the team.
	//
	// **The empty string is a value, not an absence.** It means the QR rule, which is also the
	// behaviour that existed before this field, so an unconfigured sheet keeps working as it
	// always did — and an explicit "" in an update is how an organizer switches a sheet back
	// from a post to the QR rule. Hence a pointer to the id type: absent (nil) means unchanged,
	// present-and-empty means the QR rule. Never render "" as "unknown", and never substitute
	// some other policy for it.
	//
	// HQ does not validate the id on write, and nothing re-publishes the sheet if the checkgroup
	// is later deleted, so an id that does not resolve against our own checkgroup projection
	// must be treated as "" — see the querier. The alternative is a reveal keyed to a post that
	// will never be reached, whose checkpoints would then never appear at all.
	HandoutCheckgroupID *types.CheckgroupID `json:"handoutCheckgroupId,omitempty"`
}

// Deleted records that a sheet is no longer printed.
//
// Its checkpoints are untouched: they exist independently and are almost certainly drawn on
// another sheet too.
type Deleted struct {
	KortID KortID `json:"kortId"`
}

// Sorted reorders sheets, on NATHEJK.{year}.kort.sorted.
//
// Position in the list is the sheet's sortOrder — handout order along the route, meaningful
// within a set. One event for the whole collection rather than one update per sheet, because a
// drag is one operator gesture and N events would let a replay observe orders that never existed
// on screen. **Ids not named keep their current order**, so this is not a full ordering of the
// year, and a consumer must not read absence as "move to the end".
type Sorted struct {
	KortIDs []KortID `json:"kortIds"`
}
