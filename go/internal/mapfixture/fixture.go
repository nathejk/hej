// Package mapfixture holds a single, coherent fixture world for PRD 016's map feature, and builds a real
// `reveal.Rule` over it.
//
// # Why this exists
//
// Almost every state this feature has is unreachable in development without a race actually happening: a
// relative window with an anchoring scan, a sheet reassigned to a successor team, a skitse revealed at a
// post, a scan nobody can attribute, and — the one that matters most — a positioned checkpoint that no rule
// reveals and which must therefore never appear. Without fixtures, the only way to look at any of that is
// to wait for the event, which is exactly when nobody can afford to be debugging it (task 270).
//
// # Why the real rule over fake projections, not a fake rule
//
// A hand-written `data.MapReads` mock would let us *assert* whatever states we liked, and would tell us
// nothing about whether the reveal rule produces them. So this fakes only the five projections and runs
// the **actual** `reveal.Rule` over them. In particular, `cp-secret` is absent from the dev map because the
// rule filters it, not because a mock declined to mention it — which is the only version of that check
// worth having.
//
// # How it cannot leak into production
//
// It is chosen by the **absence of the projections**, not by an environment flag — the same rule
// `scanSourceFor` follows, and for the same reason spelled out there: a misconfigured production deployment
// gets an empty map and a log line, rather than silently serving a fixture world to real patrols. A
// deployment with a database always has the projections, so it always gets the real rule. Fixture data
// reaching a race would be worse than no data, because it looks right.
//
// # Kept in step with the scans mock
//
// In the no-database mode `/api/patrol/scans` is served by `internal/scans`' mock while the map's reveal
// comes from here, so the two must agree about checkpoint ids or the drawer and the map would disagree
// about which posts have been visited. The checkpoint ids and coordinates below deliberately match
// `scans.NewMockSource`; change one and change the other.
package mapfixture

import (
	"github.com/nathejk/shared-go/types"

	"nathejk.dk/internal/reveal"
	"nathejk.dk/nathejk/table/checkgroup"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/kort"
	"nathejk.dk/nathejk/table/maphandout"
	"nathejk.dk/nathejk/table/scan"
)

// PatrolID is the fixture patrol: the spejder in the mock user directory (users.MockSpejderPatrolID).
//
// Duplicated as a literal rather than imported, so this package does not depend on the user directory —
// the same one-way-seam reasoning `internal/scans` states. A drift here presents as "this patrol has no
// map", which the coverage test catches.
const PatrolID = "mock-patrol-1042"

// The fixture evening, in unix seconds. Fixed instants, never "now": a fixture that moved with the clock
// would make the on-time verdicts change between two loads of the same page.
const (
	utsStart = 1787000000 // departure
	utsPost1 = 1787003600
	utsPost2 = 1787010800
	utsOdd   = 1787014400 // the scan nobody can attribute
)

// SecretLat and SecretLng are cp-secret's coordinates — the position that must never leave the BFF.
//
// Exported so the reveal regression test (task 258) can hunt for them in every response body without
// duplicating the fixture world. One world in one place is the point: two copies would drift, and the copy
// that drifted would be the one guarding the guarantee.
const (
	SecretLat = 56.0512
	SecretLng = 9.2033
)

// Checkgroups is the year's route, in order, with one group — cg-secret — that the patrol never reaches.
//
// The schemes are spread deliberately so every verdict path is represented: cg-1 is `fixed`, cg-2 is
// `relative` anchored on cg-1, cg-4 is `none`.
func Checkgroups() []checkgroup.Checkgroup {
	return []checkgroup.Checkgroup{
		{ID: "cg-start", Name: "Afgang", SortOrder: 0, Scheme: types.CheckgroupSchemeNone},
		{ID: "cg-1", Name: "Postlinje 1", SortOrder: 1, Scheme: types.CheckgroupSchemeFixed},
		{ID: "cg-2", Name: "Postlinje 2", SortOrder: 2,
			Scheme: types.CheckgroupSchemeRelative, RelativeCheckgroupID: "cg-1"},
		{ID: "cg-3", Name: "Postlinje 3", SortOrder: 3, Scheme: types.CheckgroupSchemeFixed},
		{ID: "cg-4", Name: "Postlinje 4", SortOrder: 4, Scheme: types.CheckgroupSchemeNone},
		// Never revealed. Present in the world so the reveal rule has something to *not* reveal.
		{ID: "cg-secret", Name: "Postlinje 9", SortOrder: 9, Scheme: types.CheckgroupSchemeFixed},
	}
}

// Checkpoints is every post in the fixture year — including the ones the patrol may not see.
//
// Coordinates sit in central Jutland so they land inside the map's default view, and cp-1..cp-4 match the
// registrations in `scans.NewMockSource` so a scanned post shows as visited.
func Checkpoints() []checkpoint.Checkpoint {
	return []checkpoint.Checkpoint{
		// Revealed twice over: drawn on a held sheet *and* in a checkgroup the patrol has scanned.
		{ID: "cp-1", Name: "Post 1 – Silkeborg Sønderskov", Checkgroup: "cg-1", SortOrder: 0,
			Lat: 56.1382, Lng: 9.5521, OpenFromUts: utsPost1 - 1800, OpenUntilUts: utsPost1 + 1800},
		// In the `relative` group: its window opens at the patrol's own cg-1 scan.
		{ID: "cp-2", Name: "Post 2 – Kløvermarken", Checkgroup: "cg-2", SortOrder: 0,
			Lat: 56.1804, Lng: 9.4812, OpenDuration: 150},
		// Revealed by a *skitse* handed over at cg-2 — no QR code, so a synthesised handout.
		{ID: "cp-3", Name: "Post 3 – Ans Bro", Checkgroup: "cg-3", SortOrder: 0,
			Lat: 56.2311, Lng: 9.5216, OpenFromUts: utsPost2 + 1800, OpenUntilUts: utsPost2 + 7200},
		// Revealed by a sheet the patrol has since had reassigned away. Revealing is monotonic, so it
		// stays revealed (PRD 016 §11.4) — the state this fixture exists to make visible.
		{ID: "cp-4", Name: "Post 4 – Gjern Bakker", Checkgroup: "cg-4", SortOrder: 0,
			Lat: 56.2609, Lng: 9.4103},
		// Revealed, but the organizers have not sited it: no position, so it is drawn nowhere and
		// arrowed at nothing. The projections filter it out, which is the honest rendering.
		{ID: "cp-5", Name: "Post 5 – endnu ikke placeret", Checkgroup: "cg-4", SortOrder: 1},
		// **Revealed by nothing.** Positioned, real, and listed on a sheet the patrol has never held.
		// It must never reach the client; the coverage test asserts exactly that.
		{ID: "cp-secret", Name: "Post 9 – hemmelig", Checkgroup: "cg-secret", SortOrder: 0,
			Lat: SecretLat, Lng: SecretLng},
	}
}

// Sheets is the patrol map set. Every reveal trigger is represented.
func Sheets() []kort.Sheet {
	return []kort.Sheet{
		// The QR rule: revealed because the patrol scanned this sheet's code.
		{ID: "kort-1", KortsaetID: "saet-patrulje", Name: "Etape 1", Format: kort.FormatA4, SortOrder: 0,
			CheckpointIDs: []types.CheckpointID{"cp-1", "cp-2"}},
		// Handed over at a post: revealed once the patrol reaches cg-2. A skitse has no QR at all, so no
		// binding event can ever name it and the handout must be synthesised.
		{ID: "kort-2", KortsaetID: "saet-patrulje", Name: "Skitse til etape 3", Format: kort.FormatSkitse,
			SortOrder: 1, HandoutCheckgroupID: "cg-2", CheckpointIDs: []types.CheckpointID{"cp-3"}},
		// Bound, then reassigned. Its checkpoints stay revealed.
		{ID: "kort-3", KortsaetID: "saet-patrulje", Name: "Etape 4", Format: kort.FormatA4, SortOrder: 2,
			CheckpointIDs: []types.CheckpointID{"cp-4", "cp-5"}},
		// Never handed to this patrol. This is the sheet that lists cp-secret, and the reveal rule must
		// not follow it.
		{ID: "kort-secret", KortsaetID: "saet-patrulje", Name: "Etape 9", Format: kort.FormatA4,
			SortOrder: 9, CheckpointIDs: []types.CheckpointID{"cp-secret"}},
	}
}

// Handouts are the QR bindings for the fixture patrol, oldest first.
//
// kort-2 is deliberately absent: it is a skitse, it has no code, and its handout is *synthesised* from the
// patrol reaching cg-2. That is the case a list built only from bindings would silently lose.
func Handouts() []maphandout.Handout {
	return []maphandout.Handout{
		{QrID: "1042", MapID: "kort-1", FirstUts: utsStart, LastUts: utsStart, Current: true},
		// Reassigned: renders as "afleveret", names nobody, and its checkpoints stay revealed.
		{QrID: "1055", MapID: "kort-3", FirstUts: utsPost1, LastUts: utsPost1, Current: false},
		// A code registered before its sheet was recorded: unknown sheet, not no sheet. Renders as
		// "Ukendt kort" rather than being dropped — the patrol is holding something.
		{QrID: "9999", MapID: "", FirstUts: utsPost2, LastUts: utsPost2, Current: true},
	}
}

// Scans are the fixture patrol's scans as the *projection* reports them, window and scheme included.
//
// These drive the reveal rule (which checkgroups have been reached) and carry the fields the verdict is
// computed from. Note the last row: a scan no shift could place, which must still be listed — with no
// checkpoint and no verdict.
func Scans() []scan.Scan {
	return []scan.Scan{
		{QrID: "1042", Uts: utsPost1, ScannerID: "mock-post-1",
			CheckpointID: "cp-1", CheckpointName: "Post 1 – Silkeborg Sønderskov", CheckgroupID: "cg-1",
			OpenFromUts: utsPost1 - 1800, OpenUntilUts: utsPost1 + 1800,
			Scheme: string(types.CheckgroupSchemeFixed)},
		// The relative case with an anchor present: this group's window opens at the cg-1 scan above.
		{QrID: "1042", Uts: utsPost2, ScannerID: "mock-post-2",
			CheckpointID: "cp-2", CheckpointName: "Post 2 – Kløvermarken", CheckgroupID: "cg-2",
			OpenDuration: 150, Scheme: string(types.CheckgroupSchemeRelative),
			RelativeCheckgroupID: "cg-1"},
		// Unattributable: nobody was on a recorded shift, so there is no checkpoint and no verdict.
		{QrID: "1042", Uts: utsOdd, ScannerID: "mock-unknown"},
	}
}

// NewRule builds a real reveal.Rule over the fixture world.
//
// Everything faked is data; the rule itself is the production one, so the dev map shows what the reveal
// logic actually does rather than what a mock was told to say.
func NewRule() *reveal.Rule {
	return reveal.New(
		fixtureSheets{sheets: Sheets()},
		fixtureHandouts{handouts: Handouts()},
		fixtureScans{scans: Scans()},
		fixtureCheckpoints{all: Checkpoints()},
		fixtureCheckgroups{groups: Checkgroups()},
	)
}

// The five projection fakes. Each honours its real counterpart's contract — in particular
// fixtureCheckpoints is *bounded*: it returns only what it was asked for, because the security boundary
// lives in that read (PRD 016 §6) and a fake that ignored the filter would test nothing.

type fixtureSheets struct{ sheets []kort.Sheet }

func (f fixtureSheets) PatrolSheets(string) ([]kort.Sheet, error) { return f.sheets, nil }

type fixtureHandouts struct{ handouts []maphandout.Handout }

// Keyed by team, so a patrol that is not the fixture patrol correctly has no sheets.
func (f fixtureHandouts) ByPatrol(_ string, teamID types.TeamID) ([]maphandout.Handout, error) {
	if string(teamID) != PatrolID {
		return []maphandout.Handout{}, nil
	}
	return f.handouts, nil
}

type fixtureScans struct{ scans []scan.Scan }

func (f fixtureScans) ByTeam(_ string, teamID string) ([]scan.Scan, error) {
	if teamID != PatrolID {
		return []scan.Scan{}, nil
	}
	return f.scans, nil
}

type fixtureCheckgroups struct{ groups []checkgroup.Checkgroup }

func (f fixtureCheckgroups) ByYear(string) ([]checkgroup.Checkgroup, error) { return f.groups, nil }

// fixtureCheckpoints holds every checkpoint in the year, including the one no rule reveals. Positioned
// checkpoints only come back, mirroring the real projection: an unsited post has nothing to draw.
type fixtureCheckpoints struct{ all []checkpoint.Checkpoint }

func (f fixtureCheckpoints) ByIDs(_ string, ids []types.CheckpointID) ([]checkpoint.Checkpoint, error) {
	want := make(map[types.CheckpointID]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	return f.filter(func(c checkpoint.Checkpoint) bool { return want[c.ID] }), nil
}

func (f fixtureCheckpoints) ByCheckgroups(_ string, gs []types.CheckgroupID) ([]checkpoint.Checkpoint, error) {
	want := make(map[types.CheckgroupID]bool, len(gs))
	for _, g := range gs {
		want[g] = true
	}
	return f.filter(func(c checkpoint.Checkpoint) bool { return want[c.Checkgroup] }), nil
}

// filter applies the caller's predicate and drops unpositioned posts, as the real queries do.
func (f fixtureCheckpoints) filter(keep func(checkpoint.Checkpoint) bool) []checkpoint.Checkpoint {
	out := []checkpoint.Checkpoint{}
	for _, c := range f.all {
		if !keep(c) {
			continue
		}
		// 0,0 is the Atlantic off Ghana and is what an unset coordinate serialises to. An unsited post
		// has nothing to draw and nothing to point an arrow at, so it is absent rather than returned.
		if c.Lat == 0 && c.Lng == 0 {
			continue
		}
		out = append(out, c)
	}
	return out
}
