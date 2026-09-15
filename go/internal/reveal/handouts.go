package reveal

import (
	"sort"

	"github.com/nathejk/shared-go/types"

	"nathejk.dk/nathejk/table/kort"
	"nathejk.dk/nathejk/table/scan"
)

// Handout is one map sheet this patrol has been given, as the app shows it.
type Handout struct {
	// Sheet is the kort id, or "" when the QR was registered before its sheet was recorded.
	//
	// "" means *unknown sheet*, not *no sheet*: the patrol is holding something, and the app says so with
	// "Ukendt kort". Dropping the row would hide a sheet that is in their hand.
	Sheet kort.KortID
	Name  string
	// Format is a4 | a3 | skitse | andet, empty when the sheet is unknown.
	Format kort.Format

	// QrID is the printed sticker number, or "" for a synthesised handout.
	//
	// Worth showing a participant, unlike most internal ids: it is printed on the sheet, so it is the one
	// identifier a patrol can read aloud down a phone when nothing else matches.
	QrID string

	// HandedOutUts is when the patrol got it, in unix seconds — the QR binding for a bound sheet, the
	// anchoring scan for a synthesised one.
	HandedOutUts int64

	// StillHeld is false when the sheet has been reassigned to another team.
	//
	// The app renders that as "afleveret" and names nobody. Always true for a synthesised handout: there
	// is no binding to lose, so there is nothing that could have moved.
	StillHeld bool

	// Synthesised marks a handout that has no QR binding and was inferred from the patrol reaching the
	// sheet's handout post. The client uses it to lay the row out without an empty gap where the sticker
	// number would go.
	Synthesised bool
}

// Handouts returns the map sheets this patrol has been given, oldest first.
//
// # Why this lives beside the reveal rule rather than in a handler
//
// It answers "what has this patrol been given" from the same four inputs, using the same predicate the
// reveal rule already evaluates. Implemented separately, the list and the map would eventually disagree —
// and the disagreement a patrol would notice is the worst kind: posts drawn on the map for a sheet the
// list says they were never handed.
//
// # Two sources, because there are two kinds of handover
//
//  1. **QR-bound sheets** come from the handout projection: someone scanned the sheet's code and bound it
//     to the team. That is a recorded fact, with a sticker number and a timestamp.
//
//  2. **Sheets handed over at a post** are *synthesised*: they have no QR code — a skitse has none at all —
//     so no binding event can ever name them, and they would otherwise be missing from the list of things
//     the patrol is carrying. They are listed once the patrol reaches the checkgroup the sheet is handed
//     out at, which is exactly when the reveal rule starts drawing their checkpoints.
//
// A list that silently omitted a sheet in the patrol's hand would be worse than no list, because it is the
// list they would check when they cannot find themselves on the paper.
func (r *Rule) Handouts(year string, patrolID string) ([]Handout, error) {
	if patrolID == "" {
		return []Handout{}, nil
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
	groups, err := r.checkgroups.ByYear(year)
	if err != nil {
		return nil, err
	}

	byID := make(map[kort.KortID]kort.Sheet, len(sheets))
	for _, s := range sheets {
		byID[s.ID] = s
	}
	knownGroups := make(map[types.CheckgroupID]bool, len(groups))
	for _, g := range groups {
		knownGroups[g.ID] = true
	}

	out := make([]Handout, 0, len(handouts))
	listed := map[kort.KortID]bool{}

	// The recorded handovers first, so a sheet that is both bound *and* keyed to a post keeps the record
	// with the real sticker number and timestamp rather than the inferred one.
	for _, h := range handouts {
		id := kort.KortID(h.MapID)
		entry := Handout{
			Sheet:        id,
			QrID:         h.QrID,
			HandedOutUts: h.FirstUts,
			StillHeld:    h.Current,
		}
		if sheet, ok := byID[id]; ok {
			entry.Name, entry.Format = sheet.Name, sheet.Format
			listed[id] = true
		}
		// An unknown sheet ("" id) is listed without a name and never recorded as listed — there is no
		// sheet to deduplicate against, and two unknown codes are two real handouts.
		out = append(out, entry)
	}

	// Then the inferred ones.
	reachedAt := firstScanPerCheckgroup(patrolScans)
	for _, sheet := range sheets {
		if sheet.HandoutCheckgroupID == "" || listed[sheet.ID] {
			continue
		}
		// A trigger naming a group that no longer exists is the QR rule (task 256), and a sheet under the
		// QR rule that has no binding was never handed over — so there is nothing to synthesise.
		if !knownGroups[sheet.HandoutCheckgroupID] {
			continue
		}
		uts, reached := reachedAt[sheet.HandoutCheckgroupID]
		if !reached {
			continue
		}

		out = append(out, Handout{
			Sheet:        sheet.ID,
			Name:         sheet.Name,
			Format:       sheet.Format,
			HandedOutUts: uts,
			// No binding exists, so there is nothing that could have been reassigned. Reporting it as
			// held is the truth: the patrol was handed a piece of paper and nobody took it back.
			StillHeld:   true,
			Synthesised: true,
		})
		listed[sheet.ID] = true
	}

	// Oldest first: handout order is the order a patrol walked the route, which is how they think about
	// their sheets. `Sheet` breaks ties so two handouts in the same second come back stably.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].HandedOutUts != out[j].HandedOutUts {
			return out[i].HandedOutUts < out[j].HandedOutUts
		}
		return out[i].Sheet < out[j].Sheet
	})
	return out, nil
}

// firstScanPerCheckgroup records when the patrol first reached each group.
//
// First rather than last: a sheet is handed over the first time a patrol arrives at the post, and a later
// re-scan of the same group does not hand them a second copy.
func firstScanPerCheckgroup(scans []scan.Scan) map[types.CheckgroupID]int64 {
	out := map[types.CheckgroupID]int64{}
	for _, s := range scans {
		if s.CheckgroupID == "" {
			continue
		}
		cg := types.CheckgroupID(s.CheckgroupID)
		if prev, ok := out[cg]; !ok || s.Uts < prev {
			out[cg] = s.Uts
		}
	}
	return out
}
