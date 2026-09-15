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

	"nathejk.dk/nathejk/table/checkgroup"
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
//
// They also do the first half of the referential-integrity work this package needs: a sheet carries the
// checkpoint ids that were *saved*, and nothing re-publishes them when a checkpoint later disappears — in
// particular, **deleting a checkgroup emits no per-checkpoint event**, so ids inside a sheet's JSON array
// cannot be cascaded out. Because these reads return only rows that exist, a stale id simply yields
// nothing. That fix does not travel over the stream: any other consumer of the kort events has to do the
// same resolution itself.
type Checkpoints interface {
	ByIDs(year string, ids []types.CheckpointID) ([]checkpoint.Checkpoint, error)
	ByCheckgroups(year string, groups []types.CheckgroupID) ([]checkpoint.Checkpoint, error)
}

// Checkgroups reads the year's checkgroups, so a sheet's handout trigger can be resolved.
//
// The second half of the integrity work. HQ does not validate `handoutCheckgroupId` on write, and nothing
// re-publishes a sheet when the checkgroup it names is later deleted — so the id can dangle, and this is
// the only place that can notice.
type Checkgroups interface {
	ByYear(year string) ([]checkgroup.Checkgroup, error)
}

// Rule evaluates the reveal rule for a patrol.
type Rule struct {
	sheets      Sheets
	handouts    Handouts
	scans       Scans
	checkpoints Checkpoints
	checkgroups Checkgroups
}

// New builds the rule from the five projections.
func New(sheets Sheets, handouts Handouts, scans Scans, checkpoints Checkpoints, checkgroups Checkgroups) *Rule {
	return &Rule{
		sheets:      sheets,
		handouts:    handouts,
		scans:       scans,
		checkpoints: checkpoints,
		checkgroups: checkgroups,
	}
}

// RevealedMap is what a patrol may see, plus where it is going.
//
// One type rather than two calls, because both answers come from the same four reads and asking twice would
// mean a second chance for them to disagree — a checkpoint list that says one thing and a "next line" that
// says another is worse than either being wrong alone.
type RevealedMap struct {
	// Checkpoints are the posts this patrol has earned sight of, in route order.
	Checkpoints []checkpoint.Checkpoint

	// NextCheckgroup is the line the patrol is heading for, or "" when there is none.
	//
	// A *line*, not a post: a postlinje holds several posts — an A and a B — and the patrol heads for the
	// line, choosing which post when they get there. So the client arrows every revealed post in this group,
	// and the choice stays with the people walking (task 275).
	//
	// "" means no arrows: at the end of the route, or before anything is revealed.
	NextCheckgroup types.CheckgroupID
}

// Revealed returns the checkpoints this patrol may see and the line it is heading for.
//
// Empty is a normal answer, not an error: a patrol before its first handout, and every user without a
// patrol at all. An empty patrol id short-circuits, because personnel roles have none and asking for the
// sheets of team "" can only ever return nothing.
//
// Only positioned checkpoints come back — the projections filter them — because an unpositioned checkpoint
// has nothing to draw and nothing to point an arrow at. It is revealed in principle and absent in practice,
// which is the honest rendering of "the organizers have not sited it yet".
//
// `hasStarted` says whether the patrol has begun the event. It is passed in rather than derived here
// because it is a fact about a *person* (see person.HasStarted, which exists so that exactly one definition
// of "started" is in play), and this package is patrol-scoped.
func (r *Rule) Revealed(year string, patrolID string, hasStarted bool) (RevealedMap, error) {
	empty := RevealedMap{Checkpoints: []checkpoint.Checkpoint{}}
	if patrolID == "" {
		return empty, nil
	}

	sheets, err := r.sheets.PatrolSheets(year)
	if err != nil {
		return empty, err
	}
	handouts, err := r.handouts.ByPatrol(year, types.TeamID(patrolID))
	if err != nil {
		return empty, err
	}
	patrolScans, err := r.scans.ByTeam(year, patrolID)
	if err != nil {
		return empty, err
	}

	reached := reachedCheckgroups(patrolScans)
	held := heldSheetIDs(handouts)

	// Which checkgroups still exist, so a sheet whose handout trigger names a deleted one can fall back
	// to the QR rule instead of being keyed to a post that will never be reached.
	groups, err := r.checkgroups.ByYear(year)
	if err != nil {
		return empty, err
	}
	knownGroups := make(map[types.CheckgroupID]bool, len(groups))
	for _, g := range groups {
		knownGroups[g.ID] = true
	}

	// Rules 1 and 2 both yield checkpoint *ids* from sheets; rule 3 yields whole *groups*. Kept apart
	// until the end because they are answered by different reads — and because collapsing them early is
	// how the "they do not nest" property gets quietly lost.
	var ids []types.CheckpointID
	for _, sheet := range sheets {
		if !revealsFor(sheet, held, reached, knownGroups) {
			continue
		}
		ids = append(ids, sheet.CheckpointIDs...)
	}

	fromSheets, err := r.checkpoints.ByIDs(year, dedupeIDs(ids))
	if err != nil {
		return empty, err
	}
	fromGroups, err := r.checkpoints.ByCheckgroups(year, reached)
	if err != nil {
		return empty, err
	}

	revealed := merge(fromSheets, fromGroups, groupOrder(groups))
	return RevealedMap{
		Checkpoints:    revealed,
		NextCheckgroup: nextCheckgroup(groups, revealed, reached, hasStarted),
	}, nil
}

// nextCheckgroup picks the line the patrol is heading for.
//
// # Progress along the route is monotonic
//
// A line is behind the patrol if they were scanned at it **or at any later line**. The second half is what
// makes this survive real data: departing the start is recorded at check-in rather than as a scan at a post,
// and the postmandskab rota can be incomplete, so a patrol demonstrably at Postlinje 2 must not still be
// pointed back at Postlinje 1 merely because nothing attributed their earlier scan (task 275).
//
// # Starting retires the first line
//
// The earliest line in route order is the start — `Start`/`Starter` in this event, with `Mål` last — and a
// patrol using this app during the race has departed it. Without this the arrows point back at `Afgang`
// forever, because no scan will ever be attributed there.
//
// Note this is the *first line*, not a name or a magic sortOrder: matching on "Start" would break the year
// somebody renames it, and lines sharing the lowest sortOrder (both line 0 here) are all part of it.
//
// # A line with nothing to point at is skipped
//
// A line whose posts are all un-revealed or un-sited would otherwise become a dead "next" — the client would
// have a group id and draw no arrows, which looks exactly like the feature being broken. `Postlinje 3` is in
// that state in the live data today, its posts having no positions yet.
func nextCheckgroup(
	groups []checkgroup.Checkgroup,
	revealed []checkpoint.Checkpoint,
	reached []types.CheckgroupID,
	hasStarted bool,
) types.CheckgroupID {
	if len(groups) == 0 {
		return ""
	}

	// Route order. `ByYear` already sorts by (sortOrder, id), so index is position along the route.
	ordered := groups

	reachedSet := make(map[types.CheckgroupID]bool, len(reached))
	for _, id := range reached {
		reachedSet[id] = true
	}

	// The furthest line the patrol has been seen at. Everything up to and including it is behind them.
	furthest := -1
	for i, g := range ordered {
		if reachedSet[g.ID] {
			furthest = i
		}
	}

	// Having started retires the whole first line, which is every group sharing the lowest sortOrder.
	if hasStarted && furthest < 0 {
		firstOrder := ordered[0].SortOrder
		for i, g := range ordered {
			if g.SortOrder == firstOrder {
				furthest = i
			}
		}
	}

	hasDrawablePost := make(map[types.CheckgroupID]bool, len(revealed))
	for _, cp := range revealed {
		hasDrawablePost[cp.Checkgroup] = true
	}

	for i := furthest + 1; i < len(ordered); i++ {
		if hasDrawablePost[ordered[i].ID] {
			return ordered[i].ID
		}
	}
	return ""
}

// groupOrder maps each checkgroup to its position along the route.
//
// Read from the same query that established which groups exist, so it costs nothing extra.
func groupOrder(groups []checkgroup.Checkgroup) map[types.CheckgroupID]int {
	order := make(map[types.CheckgroupID]int, len(groups))
	for _, g := range groups {
		order[g.ID] = g.SortOrder
	}
	return order
}

// revealsFor reports whether a sheet has been revealed to this patrol.
//
// The two sheet rules, and which one applies is decided by the sheet's own `handoutCheckgroupId`:
//
//   - "" — the QR rule. Revealed if the patrol has *ever* been handed this sheet. Ever, not currently:
//     revealing is monotonic (see the package doc), so a sheet since reassigned still counts.
//   - a checkgroup id — revealed once the patrol has reached that group. This is how a skitse works, and
//     how any sheet handed over at a post works.
//
// Note the second case does **not** require a handout record, and cannot: a skitse has no QR code, so no
// binding event ever names it. Requiring one would make every skitse permanently invisible — the exact
// mistake the two-rules-not-one warning in the contract exists to prevent.
//
// # A trigger naming a checkgroup that no longer exists falls back to the QR rule
//
// HQ does not validate the id on write, and nothing re-publishes the sheet when the group is deleted, so
// the id can dangle. Falling back to the QR rule is the safe direction, and the asymmetry is worth being
// explicit about: the alternative is a reveal keyed to a post that will never be reached, whose
// checkpoints would therefore *never* appear — a sheet in a patrol's hand whose posts the app refuses to
// draw, forever, with nothing in any log to explain it. Falling back can at worst reveal a sheet's
// checkpoints to a patrol that was handed that sheet, which is the QR rule working as intended.
//
// HQ's own read path does the same thing, and like the checkpoint-id resolution this fix does not travel
// over the stream.
func revealsFor(
	sheet kort.Sheet,
	held map[kort.KortID]bool,
	reached []types.CheckgroupID,
	knownGroups map[types.CheckgroupID]bool,
) bool {
	trigger := sheet.HandoutCheckgroupID
	if trigger != "" && !knownGroups[trigger] {
		trigger = ""
	}

	if trigger == "" {
		return held[sheet.ID]
	}
	for _, cg := range reached {
		if cg == trigger {
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
// # Route order is sorted here, not on the client
//
// Route order is **(checkgroup order, checkpoint order)**, and only the server has both halves: a
// checkpoint carries its position within its group, while the group's own position lives in the checkgroup
// projection. An earlier draft sorted on the checkpoint's half alone and left the rest to the caller —
// which would have meant shipping the group order to the client as well, so that the frontend could redo a
// sort the server was already half-doing. Worse, a client that forgot would draw arrows towards the wrong
// "next" post while looking entirely correct.
//
// So the response is fully ordered and the client trusts it. A group with no recorded order sorts first,
// which is what a zero means and is stable rather than arbitrary.
//
// Sorting *something* deterministic matters as much as the order itself: two consecutive map loads
// disagreeing would look like a bug in the app.
func merge(a, b []checkpoint.Checkpoint, groupOrder map[types.CheckgroupID]int) []checkpoint.Checkpoint {
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
		gi, gj := groupOrder[out[i].Checkgroup], groupOrder[out[j].Checkgroup]
		if gi != gj {
			return gi < gj
		}
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out
}
