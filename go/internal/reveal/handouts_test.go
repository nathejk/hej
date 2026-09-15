package reveal

import (
	"testing"

	"github.com/nathejk/shared-go/types"

	"nathejk.dk/nathejk/table/checkgroup"
	"nathejk.dk/nathejk/table/kort"
	"nathejk.dk/nathejk/table/maphandout"
	"nathejk.dk/nathejk/table/scan"
)

func sheetNames(hs []Handout) []string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, h.Name)
	}
	return out
}

func TestHandoutsListsQRBoundSheets(t *testing.T) {
	r := New(
		fakeSheets{sheets: []kort.Sheet{
			{ID: "kort-1", Name: "Kort 1", Format: kort.FormatA4},
		}},
		fakeHandouts{handouts: []maphandout.Handout{
			{QrID: "1042", MapID: "kort-1", FirstUts: 1750000000, Current: true},
		}},
		fakeScans{},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 handout, got %v", sheetNames(got))
	}
	h := got[0]
	if h.Name != "Kort 1" || h.QrID != "1042" || h.HandedOutUts != 1750000000 {
		t.Errorf("got %+v", h)
	}
	if !h.StillHeld || h.Synthesised {
		t.Errorf("a bound sheet still held: %+v", h)
	}
}

// A sheet with no QR code cannot appear in the handout projection — no binding event can ever name it — so
// it is synthesised from the patrol reaching its handout post. Without this a skitse would be missing from
// the list of things the patrol is carrying, while its checkpoints were being drawn on the map.
func TestSkitseIsSynthesisedOnceItsPostIsReached(t *testing.T) {
	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID:                  "skitse-1",
			Name:                "Skitse, Post 5–6",
			Format:              kort.FormatSkitse,
			HandoutCheckgroupID: "cg-1",
		}}},
		fakeHandouts{}, // no binding, ever
		fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-1", Uts: 1750003600}}},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want the skitse listed, got %v", sheetNames(got))
	}
	h := got[0]
	if !h.Synthesised {
		t.Error("want it marked synthesised, so the client can lay the row out without a sticker number")
	}
	if h.QrID != "" {
		t.Errorf("a synthesised handout has no sticker number, got %q", h.QrID)
	}
	if h.HandedOutUts != 1750003600 {
		t.Errorf("handout time should be the anchoring scan, got %d", h.HandedOutUts)
	}
	if !h.StillHeld {
		t.Error("nothing can have been reassigned: there is no binding to move")
	}
}

func TestSkitseIsNotListedBeforeItsPostIsReached(t *testing.T) {
	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID:                  "skitse-1",
			Name:                "Skitse, Post 5–6",
			HandoutCheckgroupID: "cg-3",
		}}},
		fakeHandouts{},
		fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-1", Uts: 1750000000}}},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want nothing listed yet, got %v", sheetNames(got))
	}
}

// The first arrival is the handover. A later re-scan of the same group does not hand the patrol a second
// copy, so the timestamp must not drift forward.
func TestSynthesisedTimeIsTheFirstScanAtThePost(t *testing.T) {
	r := New(
		fakeSheets{sheets: []kort.Sheet{{ID: "skitse-1", HandoutCheckgroupID: "cg-1"}}},
		fakeHandouts{},
		fakeScans{scans: []scan.Scan{
			{CheckgroupID: "cg-1", Uts: 1750009999},
			{CheckgroupID: "cg-1", Uts: 1750003600},
		}},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if got[0].HandedOutUts != 1750003600 {
		t.Errorf("want the earliest scan at the post, got %d", got[0].HandedOutUts)
	}
}

// A sheet that is both QR-bound and keyed to a post must appear once, keeping the recorded sticker number
// and timestamp rather than the inferred ones.
func TestSheetWithBothTriggersIsListedOnce(t *testing.T) {
	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID:                  "kort-1",
			Name:                "Kort 1",
			HandoutCheckgroupID: "cg-1",
		}}},
		fakeHandouts{handouts: []maphandout.Handout{
			{QrID: "1042", MapID: "kort-1", FirstUts: 1750000000, Current: true},
		}},
		fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-1", Uts: 1750003600}}},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want exactly one row, got %d: %v", len(got), sheetNames(got))
	}
	if got[0].QrID != "1042" || got[0].Synthesised {
		t.Errorf("want the recorded handover kept, got %+v", got[0])
	}
}

// "" is *unknown sheet*, not *no sheet*: a code registered before its sheet was written down. The patrol is
// holding something, so it is listed — the endpoint renders it as "Ukendt kort".
func TestUnknownSheetIsStillListed(t *testing.T) {
	r := New(
		fakeSheets{},
		fakeHandouts{handouts: []maphandout.Handout{
			{QrID: "1043", MapID: "", FirstUts: 1750000000, Current: true},
		}},
		fakeScans{},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("an unknown sheet must still be listed, got %d", len(got))
	}
	if got[0].Name != "" || got[0].QrID != "1043" {
		t.Errorf("got %+v", got[0])
	}
}

// Two unknown codes are two real handouts, so they must not be collapsed into one by their shared "" id.
func TestTwoUnknownSheetsAreTwoHandouts(t *testing.T) {
	r := New(
		fakeSheets{},
		fakeHandouts{handouts: []maphandout.Handout{
			{QrID: "1043", MapID: "", FirstUts: 1750000000},
			{QrID: "1044", MapID: "", FirstUts: 1750003600},
		}},
		fakeScans{},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 handouts, got %d", len(got))
	}
}

// A reassigned sheet stays in the history, marked as no longer held. Note this does not un-reveal its
// checkpoints — revealing is monotonic — so the list and the map say different but consistent things: "you
// gave this back" and "you still know where those posts are".
func TestReassignedSheetIsListedAsNoLongerHeld(t *testing.T) {
	r := New(
		fakeSheets{sheets: []kort.Sheet{{ID: "kort-1", Name: "Kort 1"}}},
		fakeHandouts{handouts: []maphandout.Handout{
			{QrID: "1042", MapID: "kort-1", FirstUts: 1750000000, Current: false},
		}},
		fakeScans{},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if got[0].StillHeld {
		t.Error("want it marked as no longer held")
	}
}

// A trigger naming a deleted group is the QR rule (task 256), so a sheet under it with no binding was never
// handed over — there is nothing to synthesise, and inventing a handout would put a sheet in the patrol's
// list that nobody gave them.
func TestDanglingTriggerSynthesisesNothing(t *testing.T) {
	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID:                  "kort-1",
			Name:                "Kort 1",
			HandoutCheckgroupID: "cg-deleted",
		}}},
		fakeHandouts{},
		fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-1", Uts: 1750000000}}},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-1"}}},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want nothing synthesised, got %v", sheetNames(got))
	}
}

func TestHandoutsAreOldestFirst(t *testing.T) {
	r := New(
		fakeSheets{sheets: []kort.Sheet{
			{ID: "kort-1", Name: "Kort 1"},
			{ID: "kort-2", Name: "Kort 2"},
		}},
		fakeHandouts{handouts: []maphandout.Handout{
			{QrID: "1044", MapID: "kort-2", FirstUts: 1750003600},
			{QrID: "1042", MapID: "kort-1", FirstUts: 1750000000},
		}},
		fakeScans{},
		&fakeCheckpoints{all: world()},
		fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if got[0].Name != "Kort 1" || got[1].Name != "Kort 2" {
		t.Errorf("want handout order, got %v", sheetNames(got))
	}
}

func TestHandoutsEmptyForAPatrolWithNothing(t *testing.T) {
	r := New(fakeSheets{}, fakeHandouts{}, fakeScans{}, &fakeCheckpoints{}, fakeCheckgroups{})

	got, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if got == nil {
		t.Fatal("want an empty slice, not nil")
	}
	if len(got) != 0 {
		t.Fatalf("got %v", sheetNames(got))
	}
}

func TestHandoutsEmptyWithoutAPatrol(t *testing.T) {
	r := New(
		fakeSheets{sheets: []kort.Sheet{{ID: "kort-1"}}},
		fakeHandouts{handouts: []maphandout.Handout{{QrID: "1042", MapID: "kort-1"}}},
		fakeScans{}, &fakeCheckpoints{}, fakeCheckgroups{},
	)

	got, err := r.Handouts("2026", "")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("personnel have no patrol and no handouts, got %v", sheetNames(got))
	}
}

// The list and the map must agree about what the patrol was given. Asserted together, because they are
// computed from the same predicate for exactly this reason — implemented separately they would drift, and
// the drift a patrol would notice is posts drawn for a sheet the list says they never had.
func TestHandoutListAgreesWithTheRevealRule(t *testing.T) {
	sheets := fakeSheets{sheets: []kort.Sheet{
		{ID: "kort-1", Name: "Kort 1", CheckpointIDs: []types.CheckpointID{"cp-1"}},
		{ID: "skitse-1", Name: "Skitse", HandoutCheckgroupID: "cg-2",
			CheckpointIDs: []types.CheckpointID{"cp-3"}},
	}}
	handouts := fakeHandouts{handouts: []maphandout.Handout{
		{QrID: "1042", MapID: "kort-1", FirstUts: 1750000000, Current: true},
	}}
	scans := fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-2", Uts: 1750003600}}}

	r := New(sheets, handouts, scans, &fakeCheckpoints{all: world()}, fakeCheckgroups{})

	listed, err := r.Handouts("2026", "team-9")
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	revealed, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}

	if len(listed) != 2 {
		t.Fatalf("want both sheets listed, got %v", sheetNames(listed))
	}
	// cp-1 from the bound sheet, cp-3 from the skitse, and cp-2 because reaching cg-2... does not apply
	// here (cp-3 is in cg-2, so rule 3 adds it too). Either way both sheets' posts are drawn.
	if !has(revealed, "cp-1") || !has(revealed, "cp-3") {
		t.Fatalf("the map must draw the posts of both listed sheets, got %v", ids(revealed))
	}
}
