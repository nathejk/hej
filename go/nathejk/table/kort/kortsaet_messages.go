package kort

import (
	"github.com/nathejk/shared-go/types"
)

// The map-set events, on NATHEJK.{year}.kortsaet.{kortsaetId}.{created,updated,deleted} and
// NATHEJK.{year}.kortsaet.sorted.
//
// A set is a group of printed sheets — most often one for the patrols and one for everybody
// else. It exists as its own entity rather than as a string on each sheet because the team type
// is a property of the set as a whole: stored per sheet, five sheets in one set would each carry
// a copy that can disagree, and "which set is the patrol set?" becomes a question with five
// possible answers.
//
// See messages.go for why these types are mirrored here rather than imported from hq.

// SetCreated records a new set of sheets.
type SetCreated struct {
	KortsaetID KortsaetID `json:"kortsaetId"`
	Name       string     `json:"name"`

	// TeamType is nil for the ordinary crew set. See PatrolTeamType for what nil means and why
	// it is the common case.
	TeamType *types.TeamType `json:"teamType,omitempty"`
}

// SetUpdated carries the set's **whole editable state**, not a patch.
//
// This is the opposite of Updated (for sheets), and the asymmetry is deliberate rather than an
// inconsistency to tidy up. A set has two editable fields and the screen that edits them always
// submits both, so whole-record semantics costs nothing — and it escapes a genuinely nasty
// tri-state: under patch semantics, "clear the team type" and "do not touch the team type" are
// both a nil pointer, and telling them apart needs either a second boolean about the same value
// or a pointer to a pointer. Both look fine and then silently refuse to let an operator un-mark
// the patrol set.
//
// A sheet's Updated has no such luxury: eight fields, and a checkpoint picker that must save a
// checkpoint list without restating extents it never loaded.
//
// **So for a consumer: an absent or null TeamType means the set has none.** It does *not* mean
// unchanged. Getting this backwards is how a set silently keeps a team type an organizer
// removed — and since the patrol handout list is filtered on exactly this field, the symptom
// would be sheets appearing for the wrong teams.
type SetUpdated struct {
	KortsaetID KortsaetID `json:"kortsaetId"`
	Name       string     `json:"name"`

	// TeamType nil means the set is not for a specific team type — which is what an operator
	// clearing the field intends, and is the reason this event carries the whole record.
	TeamType *types.TeamType `json:"teamType,omitempty"`
}

// SetDeleted records that a set is gone.
//
// Only ever published for an **empty** set: HQ refuses to delete a set that still holds sheets,
// so there is no cascade to apply. Losing a season's map definitions to a mis-click is not worth
// the convenience, and there is no undo in an event stream a projection replays.
type SetDeleted struct {
	KortsaetID KortsaetID `json:"kortsaetId"`
}

// SetsSorted reorders the year's sets, on NATHEJK.{year}.kortsaet.sorted.
//
// Same semantics as Sorted for sheets: position in the list is the order, one event for the whole
// collection, and ids not named keep their current order.
type SetsSorted struct {
	KortsaetIDs []KortsaetID `json:"kortsaetIds"`
}

// PatrolTeamType is the team type that marks the set of sheets handed to patrols.
//
// # Why this constant exists, and why it is not "spejder"
//
// In conversation this is "the spejder set" — the maps the scouts get. But `spejder` is the
// domain's word for a *person*, not a team type: HQ **refuses** it on write, and shared-go's
// TeamType has no such value. The valid set is `patrulje`, `klan`, `crew`, `gøgler`. So the set
// we want is the one marked `patrulje`, and a filter written against "spejder" would match
// nothing, reveal nothing, and produce an app that works perfectly while showing an empty map
// (PRD 016 §11.1).
//
// Named here, once, so that mistake has only one place it could be made.
//
// # It is a filter, not a key
//
// Read it as "this set is specifically for this team type", **not** "only this team type uses
// it". Three consequences, all of which the caller must respect:
//
//  1. **Absence is the ordinary case.** The crew set covers gøglere, banditter and crew, who are
//     not one team type, so forcing a value there would mean inventing a fictional one.
//  2. **It is not unique.** Several sets may carry the same value — a year that splits its
//     patrol maps into two sets is legitimate — so collect *all* matching sets. There is no
//     "the" patrol set.
//  3. **Never match on the set's name.** Names are Danish free text an organizer may rename
//     mid-season: "Patruljer", "Patruljekort", "Patruljer nord". Matching on them will break.
const PatrolTeamType = types.TeamTypePatrulje
