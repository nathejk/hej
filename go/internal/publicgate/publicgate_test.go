package publicgate

import (
	"errors"
	"testing"
	"time"

	"github.com/nathejk/shared-go/types"

	"nathejk.dk/nathejk/table/checkgroup"
	"nathejk.dk/nathejk/table/scan"
)

const year = "2025"

// The three fakes. Deliberately dumb: the rule is what is under test, and a fake that can also fail is
// how the fail-closed cases below get exercised at all.

type fakeCheckgroups struct {
	groups []checkgroup.Checkgroup
	err    error
}

func (f fakeCheckgroups) ByYear(string) ([]checkgroup.Checkgroup, error) { return f.groups, f.err }

type fakeScans struct {
	byTeam map[string][]scan.Scan
	err    error
}

func (f fakeScans) ByTeam(_, teamID string) ([]scan.Scan, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byTeam[teamID], nil
}

type fakeClosing struct {
	uts int64
	ok  bool
	err error
}

func (f fakeClosing) LastCloses(string) (int64, bool, error) { return f.uts, f.ok, f.err }

// route is three groups in order; "cg-3" is the finish line.
func route() []checkgroup.Checkgroup {
	return []checkgroup.Checkgroup{
		{ID: "cg-1", Name: "Første", SortOrder: 10},
		{ID: "cg-2", Name: "Anden", SortOrder: 20},
		{ID: "cg-3", Name: "Mål", SortOrder: 30},
	}
}

func at(group string) scan.Scan {
	return scan.Scan{QrID: "qr-1", Uts: 1000, CheckgroupID: group, CheckpointID: "cp-" + group}
}

// frozen is a clock well before any closing instant used here.
func frozen() func() time.Time {
	return func() time.Time { return time.Unix(1_700_000_000, 0) }
}

// **The finish time comes from the gate, so there is one definition of "finished"** (PRD 011 §0b.3).
// Finding it is the same work as opening the gate, and having the page re-derive it would mean two
// definitions that can disagree.
func TestFinishedCarriesTheFinishTime(t *testing.T) {
	scans := fakeScans{byTeam: map[string][]scan.Scan{
		"team-42": {at("cg-1"), at("cg-3")},
	}}
	g := New(fakeCheckgroups{groups: route()}, scans, fakeClosing{}, WithClock(frozen()))

	v, err := g.For(year, "team-42")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if !v.Finished() {
		t.Fatal("a patrol scanned at the finish must carry a finish time")
	}
	if got := v.FinishedAt.Unix(); got != 1000 {
		t.Errorf("finish time = %d, want the scan's timestamp 1000", got)
	}
}

// **A backstop-opened page has no finish time**, and that is the state task 346 renders without a diploma.
// The two facts are independent: open does not imply finished.
func TestBackstopOpensWithoutAFinishTime(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	closed := fakeClosing{uts: now.Add(-time.Hour).Unix(), ok: true}
	scans := fakeScans{byTeam: map[string][]scan.Scan{"team-43": {at("cg-1")}}}
	g := New(fakeCheckgroups{groups: route()}, scans, closed, WithClock(func() time.Time { return now }))

	v, err := g.For(year, "team-43")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if !v.Open() {
		t.Fatal("the backstop should have opened the page")
	}
	if v.Finished() {
		t.Error("a patrol that never reached the finish must not carry a finish time")
	}
}

// The **earliest** matching scan, not the latest. A patrol re-scanned at the finish, or scanned at two
// posts in the same group, finished when it first arrived — reporting the later time would quietly add the
// minutes it stood there to its night.
func TestFinishTimeIsTheEarliestScanAtTheFinish(t *testing.T) {
	first := scan.Scan{QrID: "qr-1", Uts: 5000, CheckgroupID: "cg-3"}
	later := scan.Scan{QrID: "qr-2", Uts: 9000, CheckgroupID: "cg-3"}
	// Newest-first, as the projection returns them.
	scans := fakeScans{byTeam: map[string][]scan.Scan{"team-42": {later, first}}}
	g := New(fakeCheckgroups{groups: route()}, scans, fakeClosing{}, WithClock(frozen()))

	v, _ := g.For(year, "team-42")
	if v.FinishedAt == nil {
		t.Fatal("want a finish time")
	}
	if got := v.FinishedAt.Unix(); got != 5000 {
		t.Errorf("finish time = %d, want the earliest scan 5000", got)
	}
}

// An override is about *visibility*, not about whether the patrol reached the line — so a finished patrol
// opened by hand still carries its finish time, and an unfinished one still does not.
func TestAnOverrideDoesNotInventOrDiscardAFinishTime(t *testing.T) {
	scans := fakeScans{byTeam: map[string][]scan.Scan{
		"team-42": {at("cg-3")}, // finished
		"team-43": {at("cg-1")}, // did not
	}}
	g := New(fakeCheckgroups{groups: route()}, scans, fakeClosing{},
		WithClock(frozen()),
		WithOverride(func(string) bool { return true }))

	finished, _ := g.For(year, "team-42")
	if finished.Reason != Override {
		t.Errorf("want Override, got %q", finished.Reason)
	}
	if !finished.Finished() {
		t.Error("an overridden patrol that did finish must keep its finish time")
	}

	unfinished, _ := g.For(year, "team-43")
	if unfinished.Finished() {
		t.Error("an override must not invent a finish time")
	}
}

func TestClosedBeforeAnythingHappens(t *testing.T) {
	g := New(fakeCheckgroups{groups: route()}, fakeScans{}, fakeClosing{}, WithClock(frozen()))

	v, err := g.For(year, "team-42")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if v.Reason != Closed || v.Open() {
		t.Fatalf("a patrol with no scans and an unended race must be Closed, got %q", v.Reason)
	}
}

func TestOpensOnScanAtLastCheckgroup(t *testing.T) {
	scans := fakeScans{byTeam: map[string][]scan.Scan{
		"team-42": {at("cg-1"), at("cg-2"), at("cg-3")},
	}}
	g := New(fakeCheckgroups{groups: route()}, scans, fakeClosing{}, WithClock(frozen()))

	v, err := g.For(year, "team-42")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if v.Reason != Finished {
		t.Fatalf("a scan at the last checkgroup must open the page as Finished, got %q", v.Reason)
	}
}

// The heart of the per-patrol property: one patrol finishing must not open another's page.
func TestOneFinishDoesNotOpenAnotherPatrol(t *testing.T) {
	scans := fakeScans{byTeam: map[string][]scan.Scan{
		"team-42": {at("cg-3")},
		"team-43": {at("cg-1"), at("cg-2")},
	}}
	g := New(fakeCheckgroups{groups: route()}, scans, fakeClosing{}, WithClock(frozen()))

	if v, _ := g.For(year, "team-42"); v.Reason != Finished {
		t.Fatalf("the finished patrol must be open, got %q", v.Reason)
	}
	if v, _ := g.For(year, "team-43"); v.Reason != Closed {
		t.Fatalf("a patrol still walking must stay Closed, got %q", v.Reason)
	}
}

// Scans at every group except the last must not open the page — the mid-race case.
func TestScansShortOfTheFinishStayClosed(t *testing.T) {
	scans := fakeScans{byTeam: map[string][]scan.Scan{
		"team-42": {at("cg-1"), at("cg-2")},
	}}
	g := New(fakeCheckgroups{groups: route()}, scans, fakeClosing{}, WithClock(frozen()))

	if v, _ := g.For(year, "team-42"); v.Reason != Closed {
		t.Fatalf("scans short of the finish must stay Closed, got %q", v.Reason)
	}
}

// An unattributed scan carries no checkgroup, so it cannot satisfy trigger 1. The patrol waits for the
// backstop — which is exactly the failure mode the backstop exists for.
func TestUnattributedScanDoesNotOpenThePage(t *testing.T) {
	scans := fakeScans{byTeam: map[string][]scan.Scan{
		"team-42": {{QrID: "qr-9", Uts: 2000}},
	}}
	g := New(fakeCheckgroups{groups: route()}, scans, fakeClosing{}, WithClock(frozen()))

	if v, _ := g.For(year, "team-42"); v.Reason != Closed {
		t.Fatalf("an unattributed scan must not open the page, got %q", v.Reason)
	}
}

func TestBackstopOpensEveryPatrolWhenTheLastCheckpointHasClosed(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	closed := fakeClosing{uts: now.Add(-time.Hour).Unix(), ok: true}
	scans := fakeScans{byTeam: map[string][]scan.Scan{
		"team-43": {at("cg-1")}, // retired after the first post
	}}
	g := New(fakeCheckgroups{groups: route()}, scans, closed, WithClock(func() time.Time { return now }))

	v, err := g.For(year, "team-43")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if v.Reason != RaceOver {
		t.Fatalf("the backstop must open a non-finishing patrol as RaceOver, got %q", v.Reason)
	}
}

// Before the closing instant the backstop must not fire.
func TestBackstopDoesNotFireEarly(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	notYet := fakeClosing{uts: now.Add(time.Hour).Unix(), ok: true}
	g := New(fakeCheckgroups{groups: route()}, fakeScans{}, notYet, WithClock(func() time.Time { return now }))

	if v, _ := g.For(year, "team-42"); v.Reason != Closed {
		t.Fatalf("the backstop must not fire before the closing instant, got %q", v.Reason)
	}
}

// Finished beats RaceOver when both hold, because it is the answer that decides the diploma.
func TestFinishedWinsOverBackstop(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	closed := fakeClosing{uts: now.Add(-time.Hour).Unix(), ok: true}
	scans := fakeScans{byTeam: map[string][]scan.Scan{"team-42": {at("cg-3")}}}
	g := New(fakeCheckgroups{groups: route()}, scans, closed, WithClock(func() time.Time { return now }))

	if v, _ := g.For(year, "team-42"); v.Reason != Finished {
		t.Fatalf("a finished patrol must report Finished even after the race ended, got %q", v.Reason)
	}
}

// **The failure this package must never have.** `openUntilUts` of 0 means "not set", not midnight 1970 —
// so an absent closing instant must leave the backstop unfired rather than opening every page at once.
func TestAbsentClosingInstantDoesNotOpenEverything(t *testing.T) {
	g := New(fakeCheckgroups{groups: route()}, fakeScans{}, fakeClosing{uts: 0, ok: false}, WithClock(frozen()))

	if v, _ := g.For(year, "team-42"); v.Reason != Closed {
		t.Fatalf("an absent closing instant must not open the page, got %q", v.Reason)
	}
}

// Belt and braces: even if a caller reports ok with a zero instant, the epoch must not count as "closed".
func TestZeroClosingInstantReportedOkStillDoesNotOpenEverything(t *testing.T) {
	g := New(fakeCheckgroups{groups: route()}, fakeScans{}, fakeClosing{uts: 0, ok: true}, WithClock(frozen()))

	if v, _ := g.For(year, "team-42"); v.Open() {
		t.Fatalf("a zero closing instant reported as ok must still not open the page, got %q", v.Reason)
	}
}

func TestNoCheckgroupsIsClosedNotAnError(t *testing.T) {
	g := New(fakeCheckgroups{}, fakeScans{}, fakeClosing{}, WithClock(frozen()))

	v, err := g.For(year, "team-42")
	if err != nil {
		t.Fatalf("an empty route is a normal state, not an error: %v", err)
	}
	if v.Reason != Closed {
		t.Fatalf("no checkgroups must be Closed, got %q", v.Reason)
	}
}

func TestUnreadableProjectionsFailClosed(t *testing.T) {
	boom := errors.New("database is down")

	for name, g := range map[string]*Gate{
		"checkgroups": New(fakeCheckgroups{err: boom}, fakeScans{}, fakeClosing{}, WithClock(frozen())),
		"scans": New(fakeCheckgroups{groups: route()}, fakeScans{err: boom}, fakeClosing{},
			WithClock(frozen())),
		"closing": New(fakeCheckgroups{groups: route()}, fakeScans{}, fakeClosing{err: boom},
			WithClock(frozen())),
	} {
		v, err := g.For(year, "team-42")
		if err == nil {
			t.Fatalf("%s: an unreadable projection must surface the error", name)
		}
		if v.Open() {
			t.Fatalf("%s: an unreadable projection must fail closed, got %q", name, v.Reason)
		}
	}
}

func TestEmptyPatrolIDIsClosedNotAnError(t *testing.T) {
	g := New(fakeCheckgroups{groups: route()}, fakeScans{}, fakeClosing{}, WithClock(frozen()))

	v, err := g.For(year, "")
	if err != nil {
		t.Fatalf("a personnel user has no patrol; that is not an error: %v", err)
	}
	if v.Reason != Closed {
		t.Fatalf("an empty patrol id must be Closed, got %q", v.Reason)
	}
}

func TestOverrideOpensOnePatrolOnly(t *testing.T) {
	g := New(fakeCheckgroups{groups: route()}, fakeScans{}, fakeClosing{},
		WithClock(frozen()),
		WithOverride(func(patrolID string) bool { return patrolID == "team-42" }))

	if v, _ := g.For(year, "team-42"); v.Reason != Override {
		t.Fatalf("the overridden patrol must be open, got %q", v.Reason)
	}
	if v, _ := g.For(year, "team-43"); v.Reason != Closed {
		t.Fatalf("the override must not open any other patrol, got %q", v.Reason)
	}
}

// The override must not be reachable for an empty patrol id, or a misconfigured predicate that returns
// true for everything would publish the personnel "patrol".
func TestOverrideCannotOpenTheEmptyPatrol(t *testing.T) {
	g := New(fakeCheckgroups{groups: route()}, fakeScans{}, fakeClosing{},
		WithClock(frozen()),
		WithOverride(func(string) bool { return true }))

	if v, _ := g.For(year, ""); v.Open() {
		t.Fatalf("an empty patrol id must stay closed even under a blanket override, got %q", v.Reason)
	}
}

// The last checkgroup is the one with the highest sortOrder, not the one that happens to be first in
// the slice. ByYear sorts, so this guards the "take the last element" assumption.
func TestFinishLineIsTheHighestSortOrder(t *testing.T) {
	groups := []checkgroup.Checkgroup{
		{ID: "cg-a", SortOrder: 10},
		{ID: "cg-b", SortOrder: 20},
	}
	scans := fakeScans{byTeam: map[string][]scan.Scan{"team-42": {at("cg-a")}}}
	g := New(fakeCheckgroups{groups: groups}, scans, fakeClosing{}, WithClock(frozen()))

	if v, _ := g.For(year, "team-42"); v.Reason != Closed {
		t.Fatalf("a scan at the first group must not count as the finish, got %q", v.Reason)
	}

	scans.byTeam["team-42"] = []scan.Scan{at("cg-b")}
	g = New(fakeCheckgroups{groups: groups}, scans, fakeClosing{}, WithClock(frozen()))
	if v, _ := g.For(year, "team-42"); v.Reason != Finished {
		t.Fatalf("a scan at the last group must count as the finish, got %q", v.Reason)
	}
}

// A single-group route is degenerate but legal: its only group is also the finish.
func TestSingleGroupRouteFinishesAtItsOnlyGroup(t *testing.T) {
	groups := []checkgroup.Checkgroup{{ID: "cg-only", SortOrder: 10}}
	scans := fakeScans{byTeam: map[string][]scan.Scan{"team-42": {at("cg-only")}}}
	g := New(fakeCheckgroups{groups: groups}, scans, fakeClosing{}, WithClock(frozen()))

	if v, _ := g.For(year, "team-42"); v.Reason != Finished {
		t.Fatalf("a one-group route must finish at that group, got %q", v.Reason)
	}
}

// Guards the types.CheckgroupID comparison against a stringly-typed regression.
func TestCheckgroupIDComparisonIsExact(t *testing.T) {
	groups := []checkgroup.Checkgroup{{ID: types.CheckgroupID("cg-3"), SortOrder: 30}}
	scans := fakeScans{byTeam: map[string][]scan.Scan{"team-42": {{CheckgroupID: "cg-30"}}}}
	g := New(fakeCheckgroups{groups: groups}, scans, fakeClosing{}, WithClock(frozen()))

	if v, _ := g.For(year, "team-42"); v.Open() {
		t.Fatalf("a prefix match must not count as the finish, got %q", v.Reason)
	}
}
