package distance

import (
	"math"
	"testing"
	"time"
)

// The distance estimate (task 339). Most of these tests exist because the *plausible* implementations
// are wrong in ways that would never look wrong: a track-only sum under-reports fiftyfold, and a
// round-half-up makes the word "mindst" false.

var base = time.Date(2026, 9, 19, 20, 0, 0, 0, time.UTC)

func at(min int) time.Time { return base.Add(time.Duration(min) * time.Minute) }

func f(v float64) *float64 { return &v }

// scan at a position and a time.
func scan(lat, lng float64, min int) Scan {
	return Scan{Lat: f(lat), Lng: f(lng), At: at(min)}
}

// unpositioned is a scan a post registered by hand.
func unpositioned(min int) Scan { return Scan{At: at(min)} }

func closeTo(got, want, tol float64) bool { return math.Abs(got-want) <= tol }

// Two known coordinates, checked against a figure computed independently.
func TestBetweenIsAGreatCircleDistance(t *testing.T) {
	// Copenhagen to Aarhus, ~157 km great-circle.
	got := Between(55.6761, 12.5683, 56.1629, 10.2039)
	if !closeTo(got, 157, 2) {
		t.Errorf("Between(København, Århus) = %.1f km, want ~157", got)
	}

	// A degree of latitude is ~111.2 km anywhere.
	if got := Between(55, 12, 56, 12); !closeTo(got, 111.2, 0.5) {
		t.Errorf("one degree of latitude = %.2f km, want ~111.2", got)
	}

	// The same distance both ways, and zero for a point to itself.
	if a, b := Between(55, 12, 56, 13), Between(56, 13, 55, 12); !closeTo(a, b, 0.0001) {
		t.Errorf("distance is not symmetric: %f vs %f", a, b)
	}
	if got := Between(55.7, 12.2, 55.7, 12.2); got != 0 {
		t.Errorf("a point to itself = %f, want 0", got)
	}
}

// **The base case.** Three posts in a line, one kilometre apart in latitude terms.
func TestComputeSumsTheLegsBetweenPositionedScans(t *testing.T) {
	// ~0.009° of latitude is ~1 km.
	const oneKm = 0.008993

	est := Compute([]Scan{
		scan(55.700000, 12.200000, 0),
		scan(55.700000+oneKm, 12.200000, 60),
		scan(55.700000+2*oneKm, 12.200000, 120),
	}, nil)

	if est.Legs != 2 {
		t.Fatalf("want 2 legs, got %d", est.Legs)
	}
	if !closeTo(est.Km, 2, 0.05) {
		t.Errorf("want ~2 km, got %.3f", est.Km)
	}
	if est.RaisedLegs != 0 {
		t.Errorf("no track was given, so no leg should be raised, got %d", est.RaisedLegs)
	}
}

// Scans arrive newest-first from the projection. The sum of absolute distances is the same either way,
// which is exactly why this needs its own test — a reversed-order bug is invisible in the total.
//
// The timings are deliberately plausible for a patrol on foot. An earlier version of this test had the
// patrol covering 33 km in an hour, which the speed filter now — correctly — discards; a fixture that
// describes something nobody could do is a fixture that tests the wrong code path.
func TestComputeOrdersScansByTime(t *testing.T) {
	// A triangle: A→B→C is much shorter than A→C→B, so the *order* changes the answer.
	a := scan(55.70, 12.20, 0)
	b := scan(55.71, 12.21, 60)  // ~1.4 km in an hour
	c := scan(55.74, 12.26, 300) // ~5 km in four hours

	forward := Compute([]Scan{a, b, c}, nil)
	reversed := Compute([]Scan{c, b, a}, nil)
	shuffled := Compute([]Scan{b, c, a}, nil)

	if !closeTo(forward.Km, reversed.Km, 0.0001) || !closeTo(forward.Km, shuffled.Km, 0.0001) {
		t.Errorf("the order of the input changed the answer: %.3f / %.3f / %.3f",
			forward.Km, reversed.Km, shuffled.Km)
	}

	// And the answer must be the time-ordered route, not the input-ordered one.
	timeOrdered := Between(55.70, 12.20, 55.71, 12.21) + Between(55.71, 12.21, 55.74, 12.26)
	if !closeTo(forward.Km, timeOrdered, 0.0001) {
		t.Errorf("want the time-ordered route %.3f, got %.3f", timeOrdered, forward.Km)
	}
	if forward.VehicleLegs != 0 {
		t.Errorf("the fixture should be walkable; %d legs were read as vehicles", forward.VehicleLegs)
	}
}

// One place is not a journey, and must not read as "they did not move".
func TestComputeGivesNoFigureBelowTwoPositionedScans(t *testing.T) {
	for name, scans := range map[string][]Scan{
		"nothing at all":      nil,
		"one positioned scan": {scan(55.7, 12.2, 0)},
		"only unpositioned":   {unpositioned(0), unpositioned(60)},
		"one of each":         {scan(55.7, 12.2, 0), unpositioned(60)},
	} {
		est := Compute(scans, nil)
		if est.HasFigure() {
			t.Errorf("%s: want no figure, got %.3f km over %d legs", name, est.Km, est.Legs)
		}
		if Label(est) != "" {
			t.Errorf("%s: want no label, got %q", name, Label(est))
		}
	}
}

// A manually-registered scan cannot anchor a leg, but it is counted so the page can say the figure is
// partial.
func TestComputeCountsUnplottableScans(t *testing.T) {
	est := Compute([]Scan{
		scan(55.70, 12.20, 0),
		unpositioned(30),
		// ~8 km, and four hours later: a plausible stretch of a night walk. The gap matters — the same
		// distance an hour apart would be 8 km/h and read as a vehicle.
		scan(55.75, 12.30, 240),
	}, nil)

	if est.UnplottableScans != 1 {
		t.Errorf("want 1 unplottable scan, got %d", est.UnplottableScans)
	}
	// The two positioned ones still make one leg, measured directly between them.
	if est.Legs != 1 {
		t.Errorf("want 1 leg, got %d", est.Legs)
	}
	if !closeTo(est.Km, Between(55.70, 12.20, 55.75, 12.30), 0.0001) {
		t.Errorf("the leg should span the two positioned scans, got %.3f", est.Km)
	}
}

// Past a third unpositioned, the figure understates by more than its own tolerance and the page must
// say so.
func TestIncompleteWhenTooManyScansHaveNoPosition(t *testing.T) {
	// 2 of 6 — a third exactly, which is not "more than", so complete.
	complete := Compute([]Scan{
		scan(55.70, 12.20, 0), scan(55.71, 12.21, 10), scan(55.72, 12.22, 20),
		scan(55.73, 12.23, 30), unpositioned(40), unpositioned(50),
	}, nil)
	if complete.Incomplete {
		t.Error("a third unpositioned should not yet be called incomplete")
	}

	// 3 of 6 — past the threshold.
	incomplete := Compute([]Scan{
		scan(55.70, 12.20, 0), scan(55.71, 12.21, 10), scan(55.72, 12.22, 20),
		unpositioned(30), unpositioned(40), unpositioned(50),
	}, nil)
	if !incomplete.Incomplete {
		t.Error("half the scans unpositioned must be called incomplete")
	}
}

// **The track can only raise a leg, never lower it.** This is the property that makes the figure a
// floor, and the one a well-meaning "use the track when we have it" change would break.
func TestTrackOnlyEverRaisesALeg(t *testing.T) {
	from, to := scan(55.700, 12.200, 0), scan(55.720, 12.200, 60)
	straight := Between(55.700, 12.200, 55.720, 12.200)

	// A track that wanders east and back, so the measured path is longer.
	wandering := []Point{
		{Lat: 55.700, Lng: 12.200, At: at(0)},
		{Lat: 55.710, Lng: 12.260, At: at(20)},
		{Lat: 55.720, Lng: 12.200, At: at(60)},
	}
	raised := Compute([]Scan{from, to}, wandering)
	if raised.Km <= straight {
		t.Errorf("a wandering track should raise the leg: %.3f vs straight %.3f", raised.Km, straight)
	}
	if raised.RaisedLegs != 1 {
		t.Errorf("want 1 raised leg, got %d", raised.RaisedLegs)
	}

	// A track that covers only part of the leg measures *less* than the straight line. It must be
	// ignored — this is the common case, since the app records only while it is open.
	partial := []Point{
		{Lat: 55.700, Lng: 12.200, At: at(0)},
		{Lat: 55.702, Lng: 12.200, At: at(5)},
	}
	notLowered := Compute([]Scan{from, to}, partial)
	if !closeTo(notLowered.Km, straight, 0.0001) {
		t.Errorf("a partial track must not lower the leg: %.3f vs straight %.3f",
			notLowered.Km, straight)
	}
	if notLowered.RaisedLegs != 0 {
		t.Errorf("a partial track did not raise the leg, so RaisedLegs should be 0, got %d",
			notLowered.RaisedLegs)
	}
}

// **The fiftyfold error this package exists to avoid**, demonstrated. A real patrol's track covers a
// fraction of the night; summing it alone would report a fraction of the walk.
func TestTheScanFloorSurvivesASparseTrack(t *testing.T) {
	// Four posts, ~5 km apart: a 15 km night.
	scans := []Scan{
		scan(55.70, 12.20, 0),
		scan(55.745, 12.20, 180),
		scan(55.79, 12.20, 360),
		scan(55.835, 12.20, 540),
	}
	// A track with two points, 200 m apart, recorded in one three-minute window — which is what 2%
	// coverage actually looks like.
	sparse := []Point{
		{Lat: 55.7000, Lng: 12.2000, At: at(1)},
		{Lat: 55.7018, Lng: 12.2000, At: at(3)},
	}

	est := Compute(scans, sparse)

	if est.Km < 14 {
		t.Errorf("the scan floor should dominate a sparse track: got %.2f km, want ~15", est.Km)
	}
	// And a track-only sum would have been absurd, which is the point.
	trackOnly := Between(55.7000, 12.2000, 55.7018, 12.2000)
	if trackOnly > 1 {
		t.Fatal("the fixture's track is not sparse enough to make the point")
	}
	if est.Km < trackOnly*10 {
		t.Errorf("the estimate (%.2f) is suspiciously close to the track-only sum (%.2f)",
			est.Km, trackOnly)
	}
}

// A track point outside a leg's window must not be counted into it.
func TestTrackPointsOutsideALegAreIgnored(t *testing.T) {
	from, to := scan(55.700, 12.200, 60), scan(55.710, 12.200, 120)
	straight := Between(55.700, 12.200, 55.710, 12.200)

	// Two points far away, both before the leg began.
	outside := []Point{
		{Lat: 55.500, Lng: 12.000, At: at(0)},
		{Lat: 55.900, Lng: 12.900, At: at(10)},
	}
	est := Compute([]Scan{from, to}, outside)

	if !closeTo(est.Km, straight, 0.0001) {
		t.Errorf("points outside the leg were counted: %.3f vs straight %.3f", est.Km, straight)
	}
}

// A point recorded at the exact moment of a scan counts — which is precisely when a phone is out of a
// pocket, so excluding it would drop the most reliable samples there are.
func TestTrackWindowIncludesItsEndpoints(t *testing.T) {
	from, to := scan(55.700, 12.200, 0), scan(55.720, 12.200, 60)

	// A detour whose first and last points sit exactly on the scan times.
	track := []Point{
		{Lat: 55.700, Lng: 12.200, At: at(0)},
		{Lat: 55.710, Lng: 12.300, At: at(30)},
		{Lat: 55.720, Lng: 12.200, At: at(60)},
	}
	est := Compute([]Scan{from, to}, track)

	if est.RaisedLegs != 1 {
		t.Errorf("the detour should have raised the leg; endpoints were probably excluded")
	}
}

// Two samples are needed to measure anything. One says nothing about how a leg was walked.
func TestASingleTrackPointRaisesNothing(t *testing.T) {
	from, to := scan(55.700, 12.200, 0), scan(55.720, 12.200, 60)
	straight := Between(55.700, 12.200, 55.720, 12.200)

	est := Compute([]Scan{from, to}, []Point{{Lat: 55.900, Lng: 12.900, At: at(30)}})
	if !closeTo(est.Km, straight, 0.0001) {
		t.Errorf("a single track point must not change the leg: %.3f vs %.3f", est.Km, straight)
	}
}

// **The 2025 finding, encoded.** A leg covered faster than anybody walks is a vehicle transfer between
// sections of the course, and it must not count towards how far the patrol walked — or "mindst" would sit
// in front of a number that overstates the walk. Summing everything gave 2025 a 157.9 km maximum.
func TestVehicleLegsAreExcluded(t *testing.T) {
	// 30 km in half an hour: 60 km/h.
	driven := Compute([]Scan{
		scan(55.70, 12.20, 0),
		scan(55.97, 12.20, 30),
	}, nil)

	if driven.HasFigure() {
		t.Errorf("a 60 km/h leg must not produce a figure, got %.1f km", driven.Km)
	}
	if driven.VehicleLegs != 1 {
		t.Errorf("want 1 vehicle leg, got %d", driven.VehicleLegs)
	}
	if driven.Legs != 0 {
		t.Errorf("a vehicle leg must not be counted as a leg, got %d", driven.Legs)
	}
}

// Excluded, not capped. Capping at a walking pace would invent a walk of exactly the length the filter
// happens to allow, which is a fabrication rather than a conservative estimate.
func TestAVehicleLegContributesNothingRatherThanACappedAmount(t *testing.T) {
	walked := scan(55.700, 12.200, 0)
	nextPost := scan(55.709, 12.200, 120)  // ~1 km in two hours: a walk
	drivenFar := scan(55.970, 12.200, 150) // ~29 km in half an hour: a bus

	est := Compute([]Scan{walked, nextPost, drivenFar}, nil)

	onlyTheWalk := Between(55.700, 12.200, 55.709, 12.200)
	if !closeTo(est.Km, onlyTheWalk, 0.01) {
		t.Errorf("want only the walked leg (%.3f km), got %.3f — the vehicle leg leaked in",
			onlyTheWalk, est.Km)
	}
	// Half an hour at the 7 km/h ceiling would be 3.5 km. Nothing near that may appear.
	if est.Km > onlyTheWalk+1 {
		t.Errorf("the vehicle leg appears to have been capped rather than excluded: %.3f km", est.Km)
	}
}

// A brisk walk must not be mistaken for a vehicle. The threshold is chosen to be hard to argue with in
// this direction: no patrol is excluded for walking fast.
func TestABriskWalkIsStillAWalk(t *testing.T) {
	// 6 km in an hour: fast for a patrol at night, and still a walk.
	est := Compute([]Scan{
		scan(55.700, 12.200, 0),
		scan(55.754, 12.200, 60),
	}, nil)

	if !est.HasFigure() {
		t.Fatal("a 6 km/h leg must count")
	}
	if est.VehicleLegs != 0 {
		t.Errorf("a 6 km/h leg is not a vehicle, got %d vehicle legs", est.VehicleLegs)
	}
}

// Two scans in the same second imply an infinite speed. Treating that as a vehicle would drop a leg over
// a clock artefact — and the distance is near zero anyway, so keeping it contributes nothing.
func TestSimultaneousScansAreNotTreatedAsVehicles(t *testing.T) {
	est := Compute([]Scan{
		scan(55.700, 12.200, 0),
		scan(55.700, 12.200, 0),
		scan(55.754, 12.200, 60),
	}, nil)

	if est.VehicleLegs != 0 {
		t.Errorf("a zero-duration leg is a clock artefact, not a vehicle: got %d", est.VehicleLegs)
	}
	if est.Legs != 2 {
		t.Errorf("want 2 legs, got %d", est.Legs)
	}
}

// The speed filter must not be defeated by the track: a vehicle leg is excluded before the track is
// consulted, so a recorded bus journey cannot raise a leg that should not exist.
func TestATrackCannotResurrectAVehicleLeg(t *testing.T) {
	from, to := scan(55.70, 12.20, 0), scan(55.97, 12.20, 30)
	// A track that followed the bus.
	bus := []Point{
		{Lat: 55.70, Lng: 12.20, At: at(0)},
		{Lat: 55.84, Lng: 12.20, At: at(15)},
		{Lat: 55.97, Lng: 12.20, At: at(30)},
	}
	est := Compute([]Scan{from, to}, bus)

	if est.HasFigure() {
		t.Errorf("a recorded bus journey must not become a walked distance: %.1f km", est.Km)
	}
}

// **The label is a floor, so it rounds down.** Round-half-up would occasionally print a number the
// patrol did not certainly walk, and "mindst" in front of it would then be false.
func TestLabelRoundsDownSoMindstStaysTrue(t *testing.T) {
	cases := map[float64]string{
		23.99: "mindst ~23 km",
		24.00: "mindst ~24 km",
		24.01: "mindst ~24 km",
		24.99: "mindst ~24 km",
		1.00:  "mindst ~1 km",
		1.99:  "mindst ~1 km",
		30.5:  "mindst ~30 km",
	}
	for km, want := range cases {
		got := Label(Estimate{Km: km, Legs: 1})
		if got != want {
			t.Errorf("Label(%.2f km) = %q, want %q", km, got, want)
		}
	}
}

// No decimals, ever: a decimal implies a measurement to 100 m from a sum of straight lines between posts.
func TestLabelNeverShowsADecimal(t *testing.T) {
	for _, km := range []float64{1.5, 23.47, 24.999, 0.5} {
		label := Label(Estimate{Km: km, Legs: 1})
		for _, sep := range []string{",", "."} {
			if containsDigitAround(label, sep) {
				t.Errorf("Label(%.3f) = %q contains a decimal", km, label)
			}
		}
	}
}

// Below a kilometre there is no honest round number, so it says so in words rather than printing
// "mindst ~0 km", which reads as a bug.
func TestLabelSaysUnderOneKilometreInWords(t *testing.T) {
	for _, km := range []float64{0.001, 0.4, 0.999} {
		if got := Label(Estimate{Km: km, Legs: 1}); got != "under 1 km" {
			t.Errorf("Label(%.3f) = %q, want %q", km, got, "under 1 km")
		}
	}
}

func TestLabelIsEmptyWithoutAFigure(t *testing.T) {
	for name, e := range map[string]Estimate{
		"no legs": {Km: 12, Legs: 0},
		"no km":   {Km: 0, Legs: 3},
		"neither": {},
	} {
		if got := Label(e); got != "" {
			t.Errorf("%s: want no label, got %q", name, got)
		}
	}
}

// The label must not imply a ranking or a comparison — PRD 011 §4 rules those out, and the wording is
// where a comparison would sneak in ("længst", "nr. 3", "bedre end").
func TestLabelMakesNoComparison(t *testing.T) {
	label := Label(Estimate{Km: 24, Legs: 9})
	for _, forbidden := range []string{"længst", "bedst", "hurtigst", "nr.", "plads", "rekord"} {
		if containsFold(label, forbidden) {
			t.Errorf("the label %q implies a comparison; PRD 011 §4 rules ranking out", label)
		}
	}
}

// A zero-length leg (two scans at one post) contributes nothing and must not break the count.
func TestRepeatedScansAtOnePostAddNothing(t *testing.T) {
	est := Compute([]Scan{
		scan(55.70, 12.20, 0),
		scan(55.70, 12.20, 5),
		scan(55.75, 12.20, 60),
	}, nil)

	if est.Legs != 2 {
		t.Errorf("want 2 legs, got %d", est.Legs)
	}
	if !closeTo(est.Km, Between(55.70, 12.20, 55.75, 12.20), 0.0001) {
		t.Errorf("the zero-length leg should add nothing, got %.3f", est.Km)
	}
}

// Scans recorded in the same second keep the projection's order rather than an arbitrary one.
func TestSimultaneousScansAreStable(t *testing.T) {
	a := scan(55.70, 12.20, 0)
	b := scan(55.75, 12.25, 0)
	c := scan(55.80, 12.30, 60)

	first := Compute([]Scan{a, b, c}, nil)
	again := Compute([]Scan{a, b, c}, nil)
	if first.Km != again.Km {
		t.Errorf("two runs over the same input disagreed: %.6f vs %.6f", first.Km, again.Km)
	}
}

func containsDigitAround(s, sep string) bool {
	for i := 1; i+1 < len(s); i++ {
		if string(s[i]) == sep && isDigit(s[i-1]) && isDigit(s[i+1]) {
			return true
		}
	}
	return false
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func containsFold(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexFold(s, sub) >= 0
}

func indexFold(s, sub string) int {
	lower := func(b byte) byte {
		if b >= 'A' && b <= 'Z' {
			return b + 32
		}
		return b
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if lower(s[i+j]) != lower(sub[j]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// The implausible-leg rule (PRD 011 §0b.6, task 349).
//
// Measured on 2025: the overstating tail is not fast legs, it is **slow, enormous** ones — 38.7 km over
// 13.6 hours, and 10.4 km over 143 hours. The speed filter waves those through, because dividing by a huge
// number gives a walking pace.

func TestALegThatSpansDaysIsExcluded(t *testing.T) {
	// 10 km apart, six days between the scans: 0.07 km/h, so the speed filter is happy.
	est := Compute([]Scan{
		scan(55.700, 12.200, 0),
		scan(55.790, 12.200, 6*24*60),
	}, nil)

	if est.HasFigure() {
		t.Errorf("a leg spanning six days must not produce a figure, got %.1f km", est.Km)
	}
	if est.VehicleLegs != 1 {
		t.Errorf("want the leg counted as excluded, got %d", est.VehicleLegs)
	}
}

// **A long rest is not an implausible leg.** Patrols sleep: 2025 has six-hour legs covering two kilometres.
// Rejecting those on duration alone would discard real walking, which is why the rule needs both conditions.
func TestALongRestFollowedByAShortWalkStillCounts(t *testing.T) {
	// ~2.2 km over six hours — the shape of a patrol that slept and then walked to the next post.
	est := Compute([]Scan{
		scan(55.700, 12.200, 0),
		scan(55.720, 12.200, 6*60),
	}, nil)

	if !est.HasFigure() {
		t.Fatal("a long, short leg is a real walk and must still count")
	}
	if est.VehicleLegs != 0 {
		t.Errorf("want nothing excluded, got %d", est.VehicleLegs)
	}
	if est.Km < 2 {
		t.Errorf("want ~2.2 km, got %.2f", est.Km)
	}
}

// And the boundary in the other direction: far but quick is the *speed* filter's business, and a leg that is
// neither long nor far is nobody's.
func TestTheImplausibleLegRuleNeedsBothConditions(t *testing.T) {
	// 12 km in three hours: 4 km/h, under the walking ceiling and under the duration bound. A long day's
	// walk between two posts, and it must count.
	est := Compute([]Scan{
		scan(55.700, 12.200, 0),
		scan(55.808, 12.200, 3*60),
	}, nil)

	if !est.HasFigure() {
		t.Fatal("12 km in three hours is a walk; it must count")
	}
	if est.VehicleLegs != 0 {
		t.Errorf("want nothing excluded, got %d", est.VehicleLegs)
	}
}

// UncoveredLegs — the legs the public map draws dotted (task 354).
//
// The rule they must obey is not "looks reasonable" but **the same rule the figure uses**: a leg is covered
// when the track has at least two points inside it, which is exactly when `Compute` raises it above the
// straight line. A map that dotted a leg the number had measured would be a page nobody could explain.

func TestUncoveredLegsAreTheLegsTheTrackDoesNotCover(t *testing.T) {
	// Three positioned scans: the first leg is walked with the phone open — and wandering, so the measured
	// path is longer than the straight line — while the second is not recorded at all.
	first, second, third := scan(55.700, 12.200, 0), scan(55.720, 12.200, 60), scan(55.740, 12.200, 120)
	track := []Point{
		{Lat: 55.705, Lng: 12.200, At: at(10)},
		{Lat: 55.710, Lng: 12.230, At: at(25)},
		{Lat: 55.715, Lng: 12.200, At: at(40)},
		{Lat: 55.720, Lng: 12.200, At: at(55)},
	}

	legs := UncoveredLegs([]Scan{first, second, third}, track)

	if len(legs) != 1 {
		t.Fatalf("want 1 uncovered leg, got %d", len(legs))
	}
	if !closeTo(legs[0].From.Lat, 55.720, 0.0001) || !closeTo(legs[0].To.Lat, 55.740, 0.0001) {
		t.Errorf("the uncovered leg is the wrong one: %+v", legs[0])
	}

	// And the covered leg is the one the figure raised — the agreement between the drawing and the number.
	est := Compute([]Scan{first, second, third}, track)
	if est.RaisedLegs != 1 {
		t.Errorf("want the covered leg raised by the track, got %d raised", est.RaisedLegs)
	}
}

// **Covered is not the same as raised, and the map asks the first question.** A leg whose track wandered less
// than the straight line keeps the straight line in the figure — but it is still a leg we recorded, so it must
// not be dotted. Getting this wrong is how the map ends up claiming ignorance about ground it has a trace of.
func TestALegWithATraceIsNotDottedEvenWhenItRaisesNothing(t *testing.T) {
	scans := []Scan{scan(55.700, 12.200, 0), scan(55.740, 12.200, 60)}
	// Two samples, 200 m apart, inside a 4.4 km leg: a phone that woke up briefly.
	track := []Point{
		{Lat: 55.710, Lng: 12.200, At: at(20)},
		{Lat: 55.712, Lng: 12.200, At: at(25)},
	}

	if got := len(UncoveredLegs(scans, track)); got != 0 {
		t.Errorf("a leg with a trace must not be dotted, got %d uncovered", got)
	}
	if est := Compute(scans, track); est.RaisedLegs != 0 {
		t.Errorf("and the figure should not have been raised by it, got %d", est.RaisedLegs)
	}
}

// A single track point inside a leg leaves it **uncovered**, following `alongTrack`: one point says nothing
// about how the leg was walked, and the map should not imply it does.
func TestOneTrackPointInsideALegIsNotCoverage(t *testing.T) {
	scans := []Scan{scan(55.700, 12.200, 0), scan(55.740, 12.200, 60)}
	track := []Point{{Lat: 55.710, Lng: 12.200, At: at(20)}}

	if got := len(UncoveredLegs(scans, track)); got != 1 {
		t.Errorf("one point is not coverage; want the leg dotted, got %d uncovered", got)
	}
}

// A patrol with no track at all — the common case, at 2% coverage — has every leg dotted. That is the
// situation the feature exists for: pins alone read as missing data rather than as unmeasured travel.
func TestWithNoTrackEveryLegIsUncovered(t *testing.T) {
	scans := []Scan{
		scan(55.700, 12.200, 0),
		scan(55.720, 12.200, 60),
		scan(55.740, 12.200, 120),
	}

	if got := len(UncoveredLegs(scans, nil)); got != 2 {
		t.Errorf("want both legs uncovered, got %d", got)
	}
}

// **An unplottable scan anchors nothing**, exactly as on the page's list and in the estimate: a leg needs two
// positions. A hand-written registration must not become a dotted line to nowhere.
func TestAnUnplottableScanBreaksNoLegOpen(t *testing.T) {
	legs := UncoveredLegs([]Scan{
		scan(55.700, 12.200, 0),
		unpositioned(60),
		scan(55.740, 12.200, 120),
	}, nil)

	if len(legs) != 1 {
		t.Fatalf("want one leg between the two positioned scans, got %d", len(legs))
	}
	if !closeTo(legs[0].From.Lat, 55.700, 0.0001) || !closeTo(legs[0].To.Lat, 55.740, 0.0001) {
		t.Errorf("the leg should span the unplottable scan: %+v", legs[0])
	}
}

// **A filtered leg is still drawn.** The map's claim is about what was registered, not about what was
// counted: the patrol was scanned at both ends and we have no track between, which is true whether or not the
// distance estimate excluded the leg as a vehicle (MaxWalkingKmh) or as too long and too far (MaxLegHours).
func TestALegTheEstimateExcludedIsStillUncovered(t *testing.T) {
	// 30 km in half an hour: excluded from the figure as far too fast to walk.
	driven := []Scan{scan(55.70, 12.20, 0), scan(55.97, 12.20, 30)}

	if est := Compute(driven, nil); est.HasFigure() {
		t.Fatalf("the fixture should be excluded from the figure, got %.1f km", est.Km)
	}
	if got := len(UncoveredLegs(driven, nil)); got != 1 {
		t.Errorf("want the leg drawn anyway, got %d", got)
	}
}

func TestOnePositionedScanMakesNoLeg(t *testing.T) {
	if got := UncoveredLegs([]Scan{scan(55.70, 12.20, 0)}, nil); got != nil {
		t.Errorf("one place is not a journey, got %v", got)
	}
}

// Order is time order, not the order the projection returned them — the same normalisation `Compute` does,
// and for the same reason: the source returns scans newest-first.
func TestUncoveredLegsAreInTimeOrder(t *testing.T) {
	legs := UncoveredLegs([]Scan{
		scan(55.740, 12.200, 120),
		scan(55.700, 12.200, 0),
		scan(55.720, 12.200, 60),
	}, nil)

	if len(legs) != 2 {
		t.Fatalf("want 2 legs, got %d", len(legs))
	}
	if !closeTo(legs[0].From.Lat, 55.700, 0.0001) || !closeTo(legs[1].From.Lat, 55.720, 0.0001) {
		t.Errorf("legs are not in time order: %+v", legs)
	}
}
