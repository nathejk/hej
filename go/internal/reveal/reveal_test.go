package reveal

import (
	"errors"
	"testing"

	"github.com/nathejk/shared-go/types"

	"nathejk.dk/nathejk/table/checkgroup"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/kort"
	"nathejk.dk/nathejk/table/maphandout"
	"nathejk.dk/nathejk/table/scan"
)

// The fakes below record what they were asked for as well as what they returned. That matters here more
// than usual: the guarantee this package makes is partly about the *questions* it asks — a read bounded by
// named ids is only bounded if we actually pass the narrow list.

type fakeSheets struct {
	sheets []kort.Sheet
	err    error
}

func (f fakeSheets) PatrolSheets(string) ([]kort.Sheet, error) { return f.sheets, f.err }

type fakeHandouts struct {
	handouts []maphandout.Handout
	err      error
}

func (f fakeHandouts) ByPatrol(string, types.TeamID) ([]maphandout.Handout, error) {
	return f.handouts, f.err
}

type fakeScans struct {
	scans []scan.Scan
	err   error
}

func (f fakeScans) ByTeam(string, string) ([]scan.Scan, error) { return f.scans, f.err }

// fakeCheckgroups says which groups exist. The zero value knows about the fixture world's three groups,
// so a test only mentions it when it cares — which is what makes the dangling-trigger tests read clearly.
type fakeCheckgroups struct {
	groups []checkgroup.Checkgroup
	err    error
	empty  bool
}

func (f fakeCheckgroups) ByYear(string) ([]checkgroup.Checkgroup, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return nil, nil
	}
	if f.groups != nil {
		return f.groups, nil
	}
	return []checkgroup.Checkgroup{{ID: "cg-1"}, {ID: "cg-2"}, {ID: "cg-3"}}, nil
}

// fakeCheckpoints answers from a table of every checkpoint in the fixture world — including ones no rule
// reveals, which is the whole point: if the rule asks a broader question than it should, these show up in
// the result and the test fails.
type fakeCheckpoints struct {
	all []checkpoint.Checkpoint
	err error

	askedIDs    []types.CheckpointID
	askedGroups []types.CheckgroupID
}

func (f *fakeCheckpoints) ByIDs(_ string, ids []types.CheckpointID) ([]checkpoint.Checkpoint, error) {
	f.askedIDs = append(f.askedIDs, ids...)
	if f.err != nil {
		return nil, f.err
	}
	want := map[types.CheckpointID]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := []checkpoint.Checkpoint{}
	for _, c := range f.all {
		if want[c.ID] {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeCheckpoints) ByCheckgroups(_ string, groups []types.CheckgroupID) ([]checkpoint.Checkpoint, error) {
	f.askedGroups = append(f.askedGroups, groups...)
	if f.err != nil {
		return nil, f.err
	}
	want := map[types.CheckgroupID]bool{}
	for _, g := range groups {
		want[g] = true
	}
	out := []checkpoint.Checkpoint{}
	for _, c := range f.all {
		if want[c.Checkgroup] {
			out = append(out, c)
		}
	}
	return out, nil
}

// The fixture world: three groups, four checkpoints, and one — cp-secret — that no rule below reveals.
func world() []checkpoint.Checkpoint {
	return []checkpoint.Checkpoint{
		{ID: "cp-1", Name: "Post 1", Checkgroup: "cg-1", SortOrder: 0, Lat: 56.1, Lng: 9.5},
		{ID: "cp-2", Name: "Post 2", Checkgroup: "cg-1", SortOrder: 1, Lat: 56.2, Lng: 9.5},
		{ID: "cp-3", Name: "Post 3", Checkgroup: "cg-2", SortOrder: 2, Lat: 56.3, Lng: 9.5},
		{ID: "cp-secret", Name: "Post 9", Checkgroup: "cg-3", SortOrder: 9, Lat: 56.9, Lng: 9.9},
	}
}

func ids(m RevealedMap) []string {
	cps := m.Checkpoints
	out := make([]string, 0, len(cps))
	for _, c := range cps {
		out = append(out, string(c.ID))
	}
	return out
}

func has(m RevealedMap, id types.CheckpointID) bool {
	cps := m.Checkpoints
	for _, c := range cps {
		if c.ID == id {
			return true
		}
	}
	return false
}

// Rule 1: a sheet whose QR was bound to the patrol reveals that sheet's checkpoints.
func TestRule1QRBoundSheet(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{
			{ID: "kort-1", CheckpointIDs: []types.CheckpointID{"cp-1", "cp-2"}},
		}},
		fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: "kort-1"}}},
		fakeScans{},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got.Checkpoints) != 2 || !has(got, "cp-1") || !has(got, "cp-2") {
		t.Fatalf("want cp-1 and cp-2, got %v", ids(got))
	}
	if has(got, "cp-secret") {
		t.Error("a sheet the patrol does not hold must reveal nothing")
	}
}

// A sheet the patrol was never handed reveals nothing, even though it exists and has checkpoints. This is
// the negative half of rule 1, and the one that matters.
func TestSheetNotHandedOutRevealsNothing(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{
			{ID: "kort-secret", CheckpointIDs: []types.CheckpointID{"cp-secret"}},
		}},
		fakeHandouts{}, // nothing handed out
		fakeScans{},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got.Checkpoints) != 0 {
		t.Fatalf("want nothing revealed, got %v", ids(got))
	}
}

// Rule 2: a sheet handed out at a post is revealed once the patrol reaches that checkgroup — **without** any
// handout record, because a skitse has no QR code and can never have one. Requiring a record would make
// every skitse permanently invisible.
func TestRule2SheetHandedOutAtAPost(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID:                  "skitse-1",
			Format:              kort.FormatSkitse,
			HandoutCheckgroupID: "cg-1",
			CheckpointIDs:       []types.CheckpointID{"cp-3"},
		}}},
		fakeHandouts{}, // no QR binding exists for a skitse, ever
		fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-1"}}},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if !has(got, "cp-3") {
		t.Fatalf("reaching cg-1 must reveal the skitse handed out there, got %v", ids(got))
	}
}

// And not before. A sheet keyed to a post the patrol has not reached yet stays hidden.
func TestRule2NotBeforeTheGroupIsReached(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID:                  "skitse-1",
			HandoutCheckgroupID: "cg-3",
			CheckpointIDs:       []types.CheckpointID{"cp-secret"},
		}}},
		fakeHandouts{},
		fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-1"}}},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if has(got, "cp-secret") {
		t.Errorf("a sheet handed out at an unreached post must stay hidden, got %v", ids(got))
	}
}

// Rule 3: scanning any checkpoint reveals its whole checkgroup — including the posts in that group the
// patrol has not scanned.
func TestRule3ScanRevealsTheWholeCheckgroup(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{},
		fakeHandouts{},
		fakeScans{scans: []scan.Scan{{CheckpointID: "cp-1", CheckgroupID: "cg-1"}}},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if !has(got, "cp-1") || !has(got, "cp-2") {
		t.Fatalf("want the whole group cg-1, got %v", ids(got))
	}
	if has(got, "cp-3") || has(got, "cp-secret") {
		t.Errorf("other groups must stay hidden, got %v", ids(got))
	}
}

// The three rules overlap in practice — a patrol holds a sheet covering a group it has also scanned — and
// the result must be a set, not a list with repeats. A duplicate would render as two markers on one spot.
func TestRulesOverlapWithoutDuplicating(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{
			{ID: "kort-1", CheckpointIDs: []types.CheckpointID{"cp-1", "cp-2"}},
			// A second sheet covering cp-2 as well: adjacent sheets overlap by design.
			{ID: "kort-2", CheckpointIDs: []types.CheckpointID{"cp-2", "cp-3"}},
		}},
		fakeHandouts{handouts: []maphandout.Handout{
			{QrID: "qr-1", MapID: "kort-1"},
			{QrID: "qr-2", MapID: "kort-2"},
		}},
		fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-1"}}},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got.Checkpoints) != 3 {
		t.Fatalf("want cp-1, cp-2, cp-3 exactly once each, got %v", ids(got))
	}
	seen := map[types.CheckpointID]int{}
	for _, c := range got.Checkpoints {
		seen[c.ID]++
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("%s appears %d times", id, n)
		}
	}
}

// Monotonicity (PRD 016 §11.4). When a patrol is discontinued its scouts and sheets are reassigned, so a
// sheet legitimately changes hands — but its checkpoints stay revealed. The knowledge left with the scout,
// not the sheet, so withdrawing it achieves no secrecy while making the map lie about ground already walked.
func TestReassignedSheetStaysRevealed(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{
			{ID: "kort-1", CheckpointIDs: []types.CheckpointID{"cp-1"}},
		}},
		// Current is false: the code now belongs to another team.
		fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: "kort-1", Current: false}}},
		fakeScans{},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if !has(got, "cp-1") {
		t.Errorf("a reassigned sheet's checkpoints must stay revealed, got %v", ids(got))
	}
}

// A handout whose sheet was never recorded ("" map id) reveals nothing — there is no checkpoint list to
// reveal — but it must not be mistaken for a sheet id either. Left in the patrol's handout list by the
// endpoint; simply irrelevant here.
func TestUnknownSheetRevealsNothing(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{
			{ID: "", CheckpointIDs: []types.CheckpointID{"cp-secret"}},
		}},
		fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: ""}}},
		fakeScans{},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got.Checkpoints) != 0 {
		t.Fatalf("an unknown sheet must reveal nothing, got %v", ids(got))
	}
}

// An unattributed scan has no checkgroup, so it reveals nothing. That is an upstream rota gap presenting as
// an under-reveal, which is the safe direction — and the reason task 260 counts them.
func TestUnattributedScanRevealsNothing(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{},
		fakeHandouts{},
		fakeScans{scans: []scan.Scan{{QrID: "qr-1"}}}, // no checkpoint, no group
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got.Checkpoints) != 0 {
		t.Fatalf("want nothing revealed, got %v", ids(got))
	}
	if len(cps.askedGroups) != 0 {
		t.Errorf("no groups were reached, so none should have been asked for: %v", cps.askedGroups)
	}
}

// A patrol with nothing yet gets an empty list, not an error — the state of every patrol before the start.
func TestNothingYetIsNotAnError(t *testing.T) {
	r := New(fakeSheets{}, fakeHandouts{}, fakeScans{}, &fakeCheckpoints{all: world()}, fakeCheckgroups{})

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got.Checkpoints == nil {
		t.Fatal("want an empty slice, not nil")
	}
	if len(got.Checkpoints) != 0 {
		t.Fatalf("got %v", ids(got))
	}
}

// Personnel roles have no patrol. Short-circuited before any query, because asking for the sheets of team
// "" can only ever return nothing.
func TestNoPatrolAsksNothing(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(fakeSheets{sheets: []kort.Sheet{{ID: "kort-1"}}}, fakeHandouts{}, fakeScans{}, cps, fakeCheckgroups{})

	got, err := r.Revealed("2026", "", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got.Checkpoints) != 0 {
		t.Fatalf("want nothing, got %v", ids(got))
	}
	if len(cps.askedIDs) != 0 || len(cps.askedGroups) != 0 {
		t.Error("a patrol-less user must not produce checkpoint queries")
	}
}

// The bounded-read property, asserted on the *questions* rather than the answers: the rule must never ask
// for an id it has not established possession of. A fake that returned everything regardless would make the
// result tests pass while the real query leaked.
func TestOnlyRevealedIdsAreEverAskedFor(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{
			{ID: "kort-1", CheckpointIDs: []types.CheckpointID{"cp-1"}},
			{ID: "kort-secret", CheckpointIDs: []types.CheckpointID{"cp-secret"}},
		}},
		fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: "kort-1"}}},
		fakeScans{},
		cps,
		fakeCheckgroups{},
	)

	if _, err := r.Revealed("2026", "team-9", false); err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	for _, asked := range cps.askedIDs {
		if asked == "cp-secret" {
			t.Fatal("the rule asked for a checkpoint from a sheet the patrol does not hold")
		}
	}
}

// Route order, so the arrows and the marker list agree with each other and with themselves between loads.
func TestResultIsInRouteOrder(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{
			// Deliberately out of order in the sheet's own list, which carries no meaning.
			{ID: "kort-1", CheckpointIDs: []types.CheckpointID{"cp-3", "cp-1", "cp-2"}},
		}},
		fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: "kort-1"}}},
		fakeScans{},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	want := []string{"cp-1", "cp-2", "cp-3"}
	for i, id := range ids(got) {
		if id != want[i] {
			t.Fatalf("want %v, got %v", want, ids(got))
		}
	}
}

// A failure in any input is returned, not silently treated as "nothing revealed". An empty map is a
// legitimate state, so a swallowed error would be indistinguishable from a patrol that has scanned nothing —
// and the client would cache that emptiness offline.
func TestInputFailuresAreReturned(t *testing.T) {
	boom := errors.New("database is down")

	cases := map[string]*Rule{
		"sheets": New(fakeSheets{err: boom}, fakeHandouts{}, fakeScans{}, &fakeCheckpoints{},
			fakeCheckgroups{}),
		"handouts": New(fakeSheets{}, fakeHandouts{err: boom}, fakeScans{}, &fakeCheckpoints{},
			fakeCheckgroups{}),
		"scans": New(fakeSheets{}, fakeHandouts{}, fakeScans{err: boom}, &fakeCheckpoints{},
			fakeCheckgroups{}),
		"checkpoints": New(fakeSheets{}, fakeHandouts{}, fakeScans{}, &fakeCheckpoints{err: boom},
			fakeCheckgroups{}),
		"checkgroups": New(fakeSheets{}, fakeHandouts{}, fakeScans{}, &fakeCheckpoints{},
			fakeCheckgroups{err: boom}),
	}

	for name, r := range cases {
		if _, err := r.Revealed("2026", "team-9", false); err == nil {
			t.Errorf("%s: want the error returned rather than an empty reveal", name)
		}
	}
}

// --- Read-time referential integrity (task 256) -------------------------------------------------------
//
// Two fixes that do **not** travel over the stream, so every consumer of the kort events has to implement
// them itself. Both are about ids that were valid when they were saved and are not any more.

// A sheet's checkpointIds carry whatever was saved, and nothing re-publishes them when a checkpoint later
// disappears. The stale id must be dropped silently — it is an ordinary state of the data, not an error the
// caller can act on.
func TestUnresolvableCheckpointIDsAreDropped(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID: "kort-1",
			// cp-gone was deleted after the sheet was saved.
			CheckpointIDs: []types.CheckpointID{"cp-1", "cp-gone"},
		}}},
		fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: "kort-1"}}},
		fakeScans{},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("a stale id must not be an error: %v", err)
	}
	if len(got.Checkpoints) != 1 || !has(got, "cp-1") {
		t.Fatalf("want just cp-1, got %v", ids(got))
	}
}

// The case that motivated the whole fix: **deleting a checkgroup emits no per-checkpoint event**, so the
// ids inside a sheet's JSON array cannot be cascaded out. Resolving on read is what copes, and it does so
// without depending on the order two independent projections happen to replay in.
func TestCheckgroupDeletionRemovesItsCheckpointsFromSheets(t *testing.T) {
	// The world after cg-2 was deleted: its checkpoint cp-3 is gone from the checkpoint projection, but
	// the sheet still lists it, because no event ever told anyone.
	remaining := []checkpoint.Checkpoint{
		{ID: "cp-1", Name: "Post 1", Checkgroup: "cg-1", SortOrder: 0, Lat: 56.1, Lng: 9.5},
	}
	cps := &fakeCheckpoints{all: remaining}

	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID:            "kort-1",
			CheckpointIDs: []types.CheckpointID{"cp-1", "cp-3"},
		}}},
		fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: "kort-1"}}},
		fakeScans{},
		cps,
		fakeCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-1"}}},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if has(got, "cp-3") {
		t.Errorf("a checkpoint whose group was deleted must not be revealed, got %v", ids(got))
	}
	if !has(got, "cp-1") {
		t.Errorf("the surviving checkpoint must still be revealed, got %v", ids(got))
	}
}

// A handoutCheckgroupId naming a group that no longer exists falls back to the QR rule.
//
// The safe direction, and worth stating why: keyed to a deleted post, the sheet's checkpoints would never
// appear at all — a sheet in the patrol's hand whose posts the app refuses to draw, forever, with nothing
// in any log to explain it. Falling back can at worst reveal the sheet to a patrol that was handed it,
// which is the QR rule working as intended.
func TestDanglingHandoutCheckgroupFallsBackToTheQRRule(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	sheet := kort.Sheet{
		ID:                  "kort-1",
		HandoutCheckgroupID: "cg-deleted",
		CheckpointIDs:       []types.CheckpointID{"cp-1"},
	}

	t.Run("handed out: revealed via the QR rule", func(t *testing.T) {
		r := New(
			fakeSheets{sheets: []kort.Sheet{sheet}},
			fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: "kort-1"}}},
			fakeScans{},
			cps,
			fakeCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-1"}}}, // cg-deleted is absent
		)

		got, err := r.Revealed("2026", "team-9", false)
		if err != nil {
			t.Fatalf("Revealed: %v", err)
		}
		if !has(got, "cp-1") {
			t.Errorf("a sheet keyed to a deleted post must fall back to the QR rule, got %v", ids(got))
		}
	})

	t.Run("not handed out: still hidden", func(t *testing.T) {
		r := New(
			fakeSheets{sheets: []kort.Sheet{sheet}},
			fakeHandouts{}, // never handed out
			fakeScans{},
			cps,
			fakeCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-1"}}},
		)

		got, err := r.Revealed("2026", "team-9", false)
		if err != nil {
			t.Fatalf("Revealed: %v", err)
		}
		if len(got.Checkpoints) != 0 {
			t.Errorf("the fallback is the QR rule, not an unconditional reveal, got %v", ids(got))
		}
	})
}

// And the opposite must keep working: a trigger naming a group that *does* exist is still a post-handout
// sheet, revealed by reaching the group and not by holding a QR binding it does not have.
func TestResolvableHandoutCheckgroupStillUsesRule2(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID:                  "skitse-1",
			HandoutCheckgroupID: "cg-1",
			CheckpointIDs:       []types.CheckpointID{"cp-3"},
		}}},
		fakeHandouts{}, // no binding, as a skitse never has one
		fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-1"}}},
		cps,
		fakeCheckgroups{},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if !has(got, "cp-3") {
		t.Fatalf("want cp-3 revealed by reaching cg-1, got %v", ids(got))
	}
}

// Route order is (checkgroup order, checkpoint order), and both halves are only available on the server.
// Sorted here so the client can trust the response — a frontend redoing this sort would need the group order
// shipped to it as well, and one that forgot would point arrows at the wrong "next" post while looking
// entirely correct.
func TestRouteOrderSpansCheckgroups(t *testing.T) {
	cps := &fakeCheckpoints{all: []checkpoint.Checkpoint{
		// cp-late is early *within its group* but its group comes second along the route.
		{ID: "cp-late", Checkgroup: "cg-2", SortOrder: 0, Lat: 56.3, Lng: 9.5},
		{ID: "cp-early", Checkgroup: "cg-1", SortOrder: 5, Lat: 56.1, Lng: 9.5},
	}}
	r := New(
		fakeSheets{sheets: []kort.Sheet{{
			ID:            "kort-1",
			CheckpointIDs: []types.CheckpointID{"cp-late", "cp-early"},
		}}},
		fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: "kort-1"}}},
		fakeScans{},
		cps,
		fakeCheckgroups{groups: []checkgroup.Checkgroup{
			{ID: "cg-1", SortOrder: 0},
			{ID: "cg-2", SortOrder: 1},
		}},
	)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	want := []string{"cp-early", "cp-late"}
	for i, id := range ids(got) {
		if id != want[i] {
			t.Fatalf("want %v (group order first), got %v", want, ids(got))
		}
	}
}

// --- Which line the patrol is heading for (task 275) --------------------------------------------------
//
// The route in the live data, which these fixtures mirror: line 0 is the start (`Start` and `Starter`, both
// at sortOrder 0), then Postlinje 1..4 with an A and a B post each, then `Mål`. So "next" is a *line*, and
// the client arrows every post in it.

// A route of lines, each with the posts named.
func routeWorld() (fakeSheets, fakeCheckgroups, *fakeCheckpoints) {
	groups := fakeCheckgroups{groups: []checkgroup.Checkgroup{
		{ID: "cg-start", SortOrder: 0},
		{ID: "cg-starter", SortOrder: 0},
		{ID: "cg-1", SortOrder: 1},
		{ID: "cg-2", SortOrder: 2},
		{ID: "cg-3", SortOrder: 3},
	}}
	cps := &fakeCheckpoints{all: []checkpoint.Checkpoint{
		{ID: "afgang", Name: "Afgang", Checkgroup: "cg-starter", Lat: 56.0, Lng: 9.0},
		{ID: "1a", Name: "Post 1A", Checkgroup: "cg-1", Lat: 56.1, Lng: 9.1},
		{ID: "1b", Name: "Post 1B", Checkgroup: "cg-1", Lat: 56.1, Lng: 9.2},
		{ID: "2a", Name: "Post 2A", Checkgroup: "cg-2", Lat: 56.2, Lng: 9.1},
		{ID: "2b", Name: "Post 2B", Checkgroup: "cg-2", Lat: 56.2, Lng: 9.2},
		// cg-3's posts are not sited yet, so the projections never return them — as Postlinje 3 is in the
		// live data today.
	}}
	// One sheet revealing everything, so these tests are about progress rather than about revealing.
	sheets := fakeSheets{sheets: []kort.Sheet{{
		ID: "kort-1",
		CheckpointIDs: []types.CheckpointID{
			"afgang", "1a", "1b", "2a", "2b",
		},
	}}}
	return sheets, groups, cps
}

func heldSheet() fakeHandouts {
	return fakeHandouts{handouts: []maphandout.Handout{{QrID: "qr-1", MapID: "kort-1"}}}
}

// Before starting, the patrol is standing at the start and the first line is genuinely next.
func TestNextLineBeforeStarting(t *testing.T) {
	sheets, groups, cps := routeWorld()
	r := New(sheets, heldSheet(), fakeScans{}, cps, groups)

	got, err := r.Revealed("2026", "team-9", false)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got.NextCheckgroup != "cg-starter" {
		t.Fatalf("want the start line, got %q", got.NextCheckgroup)
	}
}

// The reported bug. Departing the start is recorded at check-in, not as a scan at a post, so no scan will
// ever be attributed to line 0 — and without this the arrows point back at `Afgang` for the whole race.
func TestStartingRetiresTheFirstLine(t *testing.T) {
	sheets, groups, cps := routeWorld()
	r := New(sheets, heldSheet(), fakeScans{}, cps, groups)

	got, err := r.Revealed("2026", "team-9", true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got.NextCheckgroup != "cg-1" {
		t.Fatalf("want Postlinje 1 once started, got %q", got.NextCheckgroup)
	}
}

// Both line-0 groups are retired together: they share the lowest sortOrder, so they are one line. Matching on
// a name would break the year somebody renames `Starter`.
func TestStartingRetiresEveryGroupOnTheFirstLine(t *testing.T) {
	sheets, groups, cps := routeWorld()
	// Give the other line-0 group a drawable post too, so it could be chosen if it were not retired.
	cps.all = append(cps.all, checkpoint.Checkpoint{
		ID: "start", Name: "Start", Checkgroup: "cg-start", Lat: 56.0, Lng: 8.9,
	})
	sheets.sheets[0].CheckpointIDs = append(sheets.sheets[0].CheckpointIDs, "start")

	r := New(sheets, heldSheet(), fakeScans{}, cps, groups)

	got, err := r.Revealed("2026", "team-9", true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got.NextCheckgroup != "cg-1" {
		t.Fatalf("both line-0 groups should be behind a started patrol, got %q", got.NextCheckgroup)
	}
}

// Scanning at a line finishes it.
func TestScanningALineAdvancesToTheNext(t *testing.T) {
	sheets, groups, cps := routeWorld()
	r := New(sheets, heldSheet(), fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-1"}}}, cps, groups)

	got, err := r.Revealed("2026", "team-9", true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got.NextCheckgroup != "cg-2" {
		t.Fatalf("want Postlinje 2, got %q", got.NextCheckgroup)
	}
}

// The monotonic rule, and the reason it exists: a patrol demonstrably at Postlinje 2 must not be pointed back
// at Postlinje 1 merely because nothing attributed their earlier scan. That is not a hypothetical — the
// postmandskab rota is fed from outside this repo and can be incomplete (task 260 counts exactly this).
func TestALaterScanRetiresTheLinesBehindIt(t *testing.T) {
	sheets, groups, cps := routeWorld()
	r := New(sheets, heldSheet(), fakeScans{scans: []scan.Scan{{CheckgroupID: "cg-2"}}}, cps, groups)

	got, err := r.Revealed("2026", "team-9", true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	// cg-1 was never scanned, and is still behind them.
	if got.NextCheckgroup == "cg-1" {
		t.Fatal("a patrol seen at Postlinje 2 must not be pointed back at Postlinje 1")
	}
	// cg-3 has no sited posts, so there is nothing further to point at.
	if got.NextCheckgroup != "" {
		t.Fatalf("want no next line, got %q", got.NextCheckgroup)
	}
}

// A line with nothing drawable is skipped rather than becoming a dead "next" — the client would hold a group
// id and draw no arrows, which looks exactly like the feature being broken.
func TestALineWithNothingDrawableIsSkipped(t *testing.T) {
	sheets, groups, cps := routeWorld()
	// Postlinje 1's posts exist but are not revealed to this patrol, so nothing of cg-1 is drawable.
	sheets.sheets[0].CheckpointIDs = []types.CheckpointID{"afgang", "2a", "2b"}

	r := New(sheets, heldSheet(), fakeScans{}, cps, groups)

	got, err := r.Revealed("2026", "team-9", true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got.NextCheckgroup != "cg-2" {
		t.Fatalf("want the first line with something to point at, got %q", got.NextCheckgroup)
	}
}

// The whole next line comes back in the checkpoint list, so the client can arrow every post in it. This is the
// other half of the report: the patrol heads for the line and chooses a post when they arrive, so the app must
// not choose for them (reversing task 273's one-arrow-per-line decision).
func TestTheNextLineHasAllItsPostsAvailable(t *testing.T) {
	sheets, groups, cps := routeWorld()
	r := New(sheets, heldSheet(), fakeScans{}, cps, groups)

	got, err := r.Revealed("2026", "team-9", true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}

	var inNext []string
	for _, cp := range got.Checkpoints {
		if cp.Checkgroup == got.NextCheckgroup {
			inNext = append(inNext, string(cp.ID))
		}
	}
	if len(inNext) != 2 {
		t.Fatalf("want both posts of the next line, got %v", inNext)
	}
}

// The end of the route: no next line, so no arrows. Not an error — it is where every patrol finishes.
func TestNoNextLineWhenEverythingIsBehind(t *testing.T) {
	sheets, groups, cps := routeWorld()
	r := New(sheets, heldSheet(), fakeScans{scans: []scan.Scan{
		{CheckgroupID: "cg-1"}, {CheckgroupID: "cg-2"},
	}}, cps, groups)

	got, err := r.Revealed("2026", "team-9", true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got.NextCheckgroup != "" {
		t.Fatalf("want no next line, got %q", got.NextCheckgroup)
	}
}

// A patrol with nothing revealed has nowhere to be pointed, and that must not be an error either.
func TestNoNextLineWithoutAnyRevealedPosts(t *testing.T) {
	_, groups, cps := routeWorld()
	r := New(fakeSheets{}, fakeHandouts{}, fakeScans{}, cps, groups)

	got, err := r.Revealed("2026", "team-9", true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got.NextCheckgroup != "" {
		t.Fatalf("want no next line, got %q", got.NextCheckgroup)
	}
}
