// Package reveal decides which checkpoints a patrol has earned sight of (PRD 016).
//
// # The rule
//
// A patrol may see exactly the checkpoints that are drawn on the map sheets it has been handed, plus the
// checkpoints of every checkgroup it has already reached. Three inputs, and the contract HQ publishes is
// explicit that they are three rather than one:
//
//  1. a sheet whose QR code was bound to the patrol reveals that sheet's checkpoints;
//  2. a sheet handed out at a post reveals its checkpoints once the patrol reaches that checkgroup;
//  3. scanning any checkpoint reveals its whole checkgroup.
//
// **They do not nest and none can be derived from another.** A skitse shows a subset of one checkgroup; a
// double-sided A3 spans two. An implementation that treated rule 1 as a special case of rule 3, or the
// reverse, would be wrong in both directions at once.
//
// # Why this is grounded in possession, and why that makes it safe
//
// Everything here follows from something the patrol physically has: a sheet in their hand, or a post they
// have stood at. A checkpoint drawn on the paper in front of somebody cannot be a secret from them. That
// is what makes the rule defensible without reference to any organizer-set visibility flag — and it is why
// `showOnMap` is deliberately not consumed anywhere in this repo (PRD 016 §11.3).
//
// It also means the rule cannot over-reveal by construction: to widen it you would have to invent
// possession that did not happen.
//
// # Revealing is monotonic
//
// Nothing already revealed is ever withdrawn, including when a sheet is reassigned to another team — which
// happens when a patrol is discontinued and its scouts move on. The knowledge left with the scout, not the
// sheet, so un-revealing would achieve no secrecy while making the map lie about ground the patrol has
// already walked (PRD 016 §11.4).
//
// # Why the rule lives here and not in the checkpoint projection
//
// PRD 016 §8 first sketched this as `checkpoint.Queries.RevealedCheckpoints`. It is not there, and the
// reason is a constraint the PRD did not weigh: the `nathejk/table/*` packages are bound for shared-go and
// must not import one another, while this rule needs four of them at once — sheets, handouts, scans and
// checkpoints. Putting it in any single projection would make that projection read another's tables.
//
// So the composition lives in internal/, and the projections keep the property that matters: their reads
// are **bounded by what the caller already names** (see checkpoint.Queries). This package decides which
// ids a patrol has earned; the projections refuse to answer any broader question. The boundary is still in
// the interface, just one layer out.
package reveal

import (
	"sort"

	"github.com/nathejk/shared-go/types"

	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/kort"
	"nathejk.dk/nathejk/table/maphandout"
	"nathejk.dk/nathejk/table/scan"
)

// Sheets reads the year's patrol map sheets.
//
// Narrow local interfaces for the four inputs, rather than taking the concrete tables: the rule is the
// interesting part and it is worth testing without a database. The types are the projections' own, so
// nothing is duplicated.
type Sheets interface {
	PatrolSheets(year string) ([]kort.Sheet, error)
}

// Handouts reads which sheets a patrol has been given.
type Handouts interface {
	ByPatrol(year string, teamID types.TeamID) ([]maphandout.Handout, error)
}

// Scans reads a patrol's scans, which is where reached checkgroups come from.
type Scans interface {
	ByTeam(year, teamID string) ([]scan.Scan, error)
}

// Checkpoints resolves ids and groups to positioned checkpoints.
//
// Both reads are bounded by what we pass in, which is what keeps an un-revealed position unreachable from
// here even by mistake.
type Checkpoints interface {
	ByIDs(year string, ids []types.CheckpointID) ([]checkpoint.Checkpoint, error)
	ByCheckgroups(year string, groups []types.CheckgroupID) ([]checkpoint.Checkpoint, error)
}

// Rule evaluates the reveal rule for a patrol.
type Rule struct {
	sheets      Sheets
	handouts    Handouts
	scans       Scans
	checkpoints Checkpoints
}

// New builds the rule from the four projections.
func New(sheets Sheets, handouts Handouts, scans Scans, checkpoints Checkpoints) *Rule {
	return &Rule{sheets: sheets, handouts: handouts, scans: scans, checkpoints: checkpoints}
}

// Revealed returns the checkpoints this patrol may see, in route order.
//
// Empty is a normal answer, not an error: a patrol before its first handout, and every user without a
// patrol at all. An empty patrol id short-circuits, because personnel roles have none and asking for the
// sheets of team "" can only ever return nothing.
//
// Only positioned checkpoints come back \u2014 the projections filter them \u2014 because an unpositioned checkpoint
// has nothing to draw and nothing to point an arrow at. It is revealed in principle and absent in practice,
// which is the honest rendering of "the organizers have not sited it yet".
func (r *Rule) Revealed(year string, patrolID string) ([]checkpoint.Checkpoint, error) {
	if patrolID == "" {
		return []checkpoint.Checkpoint{}, nil
	}

	sheets, err := r.sheets.PatrolSheets(year)
	if err != nil {
		return nil, err
	}
	handouts, err := r.handouts.ByPatrol(year, types.TeamID(patrolID))
	if err != nil {
		return nil, err
	}
	patrolScans, err := r.scans.ByTeam(year, patrolID)
	if err != nil {
		return nil, err
	}

	reached := reachedCheckgroups(patrolScans)
	held := heldSheetIDs(handouts)

	// Rules 1 and 2 both yield checkpoint *ids* from sheets; rule 3 yields whole *groups*. Kept apart
	// until the end because they are answered by different reads \u2014 and because collapsing them early is
	// how the "they do not nest" property gets quietly lost.
	var ids []types.CheckpointID
	for _, sheet := range sheets {
		if !revealsFor(sheet, held, reached) {
			continue
		}
		ids = append(ids, sheet.CheckpointIDs...)
	}

	fromSheets, err := r.checkpoints.ByIDs(year, dedupeIDs(ids))
	if err != nil {
		return nil, err
	}
	fromGroups, err := r.checkpoints.ByCheckgroups(year, reached)
	if err != nil {
		return nil, err
	}

	return merge(fromSheets, fromGroups), nil
}

// revealsFor reports whether a sheet has been revealed to this patrol.
//
// The two sheet rules, and which one applies is decided by the sheet's own `handoutCheckgroupId`:
//
//   - "" \u2014 the QR rule. Revealed if the patrol has *ever* been handed this sheet. Ever, not currently:
//     revealing is monotonic (see the package doc), so a sheet since reassigned still counts.
//   - a checkgroup id \u2014 revealed once the patrol has reached that group. This is how a skitse works, and
//     how any sheet handed over at a post works.
//
// Note the second case does **not** require a handout record, and cannot: a skitse has no QR code, so no
// binding event ever names it. Requiring one would make every skitse permanently invisible \u2014 the exact
// mistake the two-rules-not-one warning in the contract exists to prevent.
func revealsFor(sheet kort.Sheet, held map[kort.KortID]bool, reached []types.CheckgroupID) bool {
	if sheet.HandoutCheckgroupID == "" {
		return held[sheet.ID]
	}
	for _, cg := range reached {
		if cg == sheet.HandoutCheckgroupID {
			return true
		}
	}
	return false
}

// heldSheetIDs collects the sheets a patrol has ever been handed.
//
// `Current` is deliberately ignored. It tells us whether the patrol still holds the code, which matters
// for how the handout list is *labelled* ("afleveret") and not at all for what has been revealed.
func heldSheetIDs(handouts []maphandout.Handout) map[kort.KortID]bool {
	held := make(map[kort.KortID]bool, len(handouts))
	for _, h := range handouts {
		// "" means the QR was registered before its sheet was recorded \u2014 unknown sheet, not no sheet. It
		// reveals nothing, because there is no checkpoint list to reveal, but it is still a real handout
		// and appears in the patrol's list.
		if h.MapID == "" {
			continue
		}
		held[kort.KortID(h.MapID)] = true
	}
	return held
}

// reachedCheckgroups collects the groups this patrol has scanned at.
//
// Derived from attributed scans, so it inherits their limitation: a scan the personnel rota could not place
// has no checkgroup and reveals nothing. That is an upstream data gap presenting as an under-reveal \u2014 the
// safe direction, and the reason task 260 counts unattributed scans.
func reachedCheckgroups(scans []scan.Scan) []types.CheckgroupID {
	seen := map[string]bool{}
	var out []types.CheckgroupID
	for _, s := range scans {
		if s.CheckgroupID == "" || seen[s.CheckgroupID] {
			continue
		}
		seen[s.CheckgroupID] = true
		out = append(out, types.CheckgroupID(s.CheckgroupID))
	}
	return out
}

// dedupeIDs removes repeats, keeping first-seen order.
//
// A checkpoint may be drawn on any number of sheets \u2014 adjacent sheets overlap by design, and every patrol
// sheet's ground is also on the crew map \u2014 so seeing an id twice is normal rather than a bug.
func dedupeIDs(ids []types.CheckpointID) []types.CheckpointID {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[types.CheckpointID]bool, len(ids))
	out := make([]types.CheckpointID, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// merge combines the two reads into one list in route order, without duplicates.
//
// Route order is (checkgroup order, checkpoint order), but this package sees only the checkpoint's own
// sortOrder \u2014 the group's belongs to the checkgroup projection. Sorting by what we have keeps the list
// stable and grouped; the caller that needs true route order joins the group's order itself (task 263).
// Sorting *something* deterministic matters more than which: two consecutive map loads disagreeing about
// order would look like a bug in the app.
func merge(a, b []checkpoint.Checkpoint) []checkpoint.Checkpoint {
	out := make([]checkpoint.Checkpoint, 0, len(a)+len(b))
	seen := make(map[types.CheckpointID]bool, len(a)+len(b))

	for _, list := range [][]checkpoint.Checkpoint{a, b} {
		for _, c := range list {
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			out = append(out, c)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out
}
