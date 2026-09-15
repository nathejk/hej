package reveal

import (
	"errors"
	"testing"

	"github.com/nathejk/shared-go/types"

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

func ids(cps []checkpoint.Checkpoint) []string {
	out := make([]string, 0, len(cps))
	for _, c := range cps {
		out = append(out, string(c.ID))
	}
	return out
}

func has(cps []checkpoint.Checkpoint, id types.CheckpointID) bool {
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
	)

	got, err := r.Revealed("2026", "team-9")
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got) != 2 || !has(got, "cp-1") || !has(got, "cp-2") {
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
	)

	got, err := r.Revealed("2026", "team-9")
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got) != 0 {
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
	)

	got, err := r.Revealed("2026", "team-9")
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
	)

	got, err := r.Revealed("2026", "team-9")
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
	)

	got, err := r.Revealed("2026", "team-9")
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
	)

	got, err := r.Revealed("2026", "team-9")
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want cp-1, cp-2, cp-3 exactly once each, got %v", ids(got))
	}
	seen := map[types.CheckpointID]int{}
	for _, c := range got {
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
	)

	got, err := r.Revealed("2026", "team-9")
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
	)

	got, err := r.Revealed("2026", "team-9")
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got) != 0 {
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
	)

	got, err := r.Revealed("2026", "team-9")
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want nothing revealed, got %v", ids(got))
	}
	if len(cps.askedGroups) != 0 {
		t.Errorf("no groups were reached, so none should have been asked for: %v", cps.askedGroups)
	}
}

// A patrol with nothing yet gets an empty list, not an error — the state of every patrol before the start.
func TestNothingYetIsNotAnError(t *testing.T) {
	r := New(fakeSheets{}, fakeHandouts{}, fakeScans{}, &fakeCheckpoints{all: world()})

	got, err := r.Revealed("2026", "team-9")
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got == nil {
		t.Fatal("want an empty slice, not nil")
	}
	if len(got) != 0 {
		t.Fatalf("got %v", ids(got))
	}
}

// Personnel roles have no patrol. Short-circuited before any query, because asking for the sheets of team
// "" can only ever return nothing.
func TestNoPatrolAsksNothing(t *testing.T) {
	cps := &fakeCheckpoints{all: world()}
	r := New(fakeSheets{sheets: []kort.Sheet{{ID: "kort-1"}}}, fakeHandouts{}, fakeScans{}, cps)

	got, err := r.Revealed("2026", "")
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got) != 0 {
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
	)

	if _, err := r.Revealed("2026", "team-9"); err != nil {
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
	)

	got, err := r.Revealed("2026", "team-9")
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
// and the client would cache that emptiness.
func TestInputFailuresAreReturned(t *testing.T) {
	boom := errors.New("database is down")

	cases := map[string]*Rule{
		"sheets": New(fakeSheets{err: boom}, fakeHandouts{}, fakeScans{}, &fakeCheckpoints{}),
		"handouts": New(fakeSheets{}, fakeHandouts{err: boom}, fakeScans{},
			&fakeCheckpoints{}),
		"scans":       New(fakeSheets{}, fakeHandouts{}, fakeScans{err: boom}, &fakeCheckpoints{}),
		"checkpoints": New(fakeSheets{}, fakeHandouts{}, fakeScans{}, &fakeCheckpoints{err: boom}),
	}

	for name, r := range cases {
		if _, err := r.Revealed("2026", "team-9"); err == nil {
			t.Errorf("%s: want the error returned rather than an empty reveal", name)
		}
	}
}
