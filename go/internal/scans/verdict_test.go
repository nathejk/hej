package scans

import "testing"

// The verdict must agree with hq exactly (inclusive bounds, no grace), so the tests pin the boundary
// instants rather than only the comfortable middle: an off-by-one at the edge is precisely the bug that
// would make the app and the organizers' screens disagree about who was on time.

func TestFixedWindowVerdict(t *testing.T) {
	const from, until = int64(1000), int64(2000)

	cases := []struct {
		name     string
		scanUts  int64
		wantOK   bool
		wantOn   bool
		wantDelt int
	}{
		{"before the window", 900, true, false, -100},
		{"exactly at open (inclusive)", 1000, true, true, 0},
		{"inside", 1500, true, true, 0},
		{"exactly at close (inclusive)", 2000, true, true, 0},
		{"after the window", 2100, true, false, 100},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := verdictFor(c.scanUts, SchemeFixed, from, until, 0, 0, false)
			if v == nil {
				t.Fatalf("want a verdict, got none")
			}
			if v.OnTime != c.wantOn {
				t.Errorf("OnTime = %v, want %v", v.OnTime, c.wantOn)
			}
			if v.DeltaSeconds != c.wantDelt {
				t.Errorf("DeltaSeconds = %d, want %d", v.DeltaSeconds, c.wantDelt)
			}
		})
	}
}

// A fixed window with no hours recorded (task 252 stores 0, not the epoch) is not a window, so there is
// no verdict — never a "for sent" against a post whose hours were never set.
func TestFixedWithNoHoursHasNoVerdict(t *testing.T) {
	if v := verdictFor(1500, SchemeFixed, 0, 0, 0, 0, false); v != nil {
		t.Errorf("want no verdict for an hours-less post, got %+v", v)
	}
	if v := verdictFor(1500, SchemeFixed, 1000, 0, 0, 0, false); v != nil {
		t.Errorf("want no verdict when only one bound is set, got %+v", v)
	}
}

// Relative: the window opens at the anchoring scan and lasts openDuration minutes, inclusive at both ends.
func TestRelativeWindowVerdict(t *testing.T) {
	const anchor = int64(10000)
	const durMin = 30 // window is [10000, 10000+1800] = [10000, 11800]

	if v := verdictFor(11800, SchemeRelative, 0, 0, durMin, anchor, true); v == nil || !v.OnTime {
		t.Errorf("exactly at close must be on time, got %+v", v)
	}
	if v := verdictFor(9999, SchemeRelative, 0, 0, durMin, anchor, true); v == nil || v.OnTime || v.DeltaSeconds != -1 {
		t.Errorf("one second early must be late by -1, got %+v", v)
	}
	if v := verdictFor(11801, SchemeRelative, 0, 0, durMin, anchor, true); v == nil || v.OnTime || v.DeltaSeconds != 1 {
		t.Errorf("one second after close must be late by 1, got %+v", v)
	}
}

// A relative window with no anchoring scan yet has no window, so no verdict — a wrong "for sent" is worse
// than a missing one.
func TestRelativeWithoutAnchorHasNoVerdict(t *testing.T) {
	if v := verdictFor(11000, SchemeRelative, 0, 0, 30, 0, false); v != nil {
		t.Errorf("want no verdict without an anchor, got %+v", v)
	}
	// A zero/absent duration is likewise not a window.
	if v := verdictFor(11000, SchemeRelative, 0, 0, 0, 10000, true); v != nil {
		t.Errorf("want no verdict with a zero duration, got %+v", v)
	}
}

// `none` and an unrecognised scheme both yield no verdict — never guessed from a scheme we do not model.
func TestNoneAndUnknownSchemeHaveNoVerdict(t *testing.T) {
	if v := verdictFor(1500, SchemeNone, 1000, 2000, 0, 0, false); v != nil {
		t.Errorf("none must have no verdict, got %+v", v)
	}
	if v := verdictFor(1500, "quantum", 1000, 2000, 0, 0, false); v != nil {
		t.Errorf("an unknown scheme must have no verdict, got %+v", v)
	}
	if v := verdictFor(1500, "", 1000, 2000, 0, 0, false); v != nil {
		t.Errorf("an empty (unattributed) scheme must have no verdict, got %+v", v)
	}
}

// End to end through ByPatrol: a relative window must anchor on the patrol's *own* scan at the named
// checkgroup, resolved from the same set of rows. This is the wiring the pure test cannot exercise.
func TestByPatrolResolvesRelativeAnchorFromOwnScans(t *testing.T) {
	// The patrol scans checkgroup "cg-start" at uts 5000, then a relative post in "cg-leg1" that opens
	// at the cg-start scan and lasts 60 min = [5000, 8600]. A scan at 6000 is on time.
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-leg", Uts: 6000, CheckpointID: "cp-leg", CheckpointName: "Leg 1",
			CheckgroupID: "cg-leg1", Scheme: SchemeRelative, RelativeCheckgroupID: "cg-start",
			OpenDurationMinutes: 60},
		{QrID: "qr-start", Uts: 5000, CheckpointID: "cp-start", CheckpointName: "Start",
			CheckgroupID: "cg-start", Scheme: SchemeNone},
	}}

	got := NewProjectionSource(p, "2026", nil, nil).ByPatrol("team-9")

	var leg *Scan
	for i := range got {
		if got[i].CheckpointID == "cp-leg" {
			leg = &got[i]
		}
	}
	if leg == nil {
		t.Fatal("leg scan missing")
	}
	if leg.Verdict == nil {
		t.Fatal("want a verdict once the anchor scan is present")
	}
	if !leg.Verdict.OnTime {
		t.Errorf("scan inside the anchored window must be on time, got %+v", leg.Verdict)
	}
}

// Without the anchoring scan in the set, the same relative post yields no verdict.
func TestByPatrolRelativeWithoutAnchorScanHasNoVerdict(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-leg", Uts: 6000, CheckpointID: "cp-leg", CheckpointName: "Leg 1",
			CheckgroupID: "cg-leg1", Scheme: SchemeRelative, RelativeCheckgroupID: "cg-start",
			OpenDurationMinutes: 60},
	}}

	got := NewProjectionSource(p, "2026", nil, nil).ByPatrol("team-9")

	if len(got) != 1 {
		t.Fatalf("want 1 scan, got %d", len(got))
	}
	if got[0].Verdict != nil {
		t.Errorf("want no verdict without the anchoring scan, got %+v", got[0].Verdict)
	}
}

// The anchor is the *earliest* scan at the anchoring group: a later re-scan there must not push the
// window forward, or a patrol could re-open a window they already missed.
func TestByPatrolRelativeAnchorIsEarliestScan(t *testing.T) {
	// cg-start scanned twice, at 5000 and 9000. The relative window (60 min) must open at 5000, so a
	// leg scan at 8000 is late (past 5000+3600=8600? no, 8000 < 8600 -> on time). Pick 8700 to be late
	// against the earliest anchor but on time against the later one.
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-leg", Uts: 8700, CheckpointID: "cp-leg", CheckpointName: "Leg 1",
			CheckgroupID: "cg-leg1", Scheme: SchemeRelative, RelativeCheckgroupID: "cg-start",
			OpenDurationMinutes: 60},
		{QrID: "qr-start-b", Uts: 9000, CheckpointID: "cp-start", CheckpointName: "Start",
			CheckgroupID: "cg-start", Scheme: SchemeNone},
		{QrID: "qr-start-a", Uts: 5000, CheckpointID: "cp-start", CheckpointName: "Start",
			CheckgroupID: "cg-start", Scheme: SchemeNone},
	}}

	got := NewProjectionSource(p, "2026", nil, nil).ByPatrol("team-9")

	var leg *Scan
	for i := range got {
		if got[i].CheckpointID == "cp-leg" {
			leg = &got[i]
		}
	}
	if leg == nil || leg.Verdict == nil {
		t.Fatalf("want a verdict for the leg scan, got %+v", leg)
	}
	if leg.Verdict.OnTime {
		t.Errorf("window must open at the earliest anchor (5000), making 8700 late; got %+v", leg.Verdict)
	}
	if leg.Verdict.DeltaSeconds != 100 { // 8700 - (5000+3600)
		t.Errorf("delta = %d, want 100", leg.Verdict.DeltaSeconds)
	}
}
