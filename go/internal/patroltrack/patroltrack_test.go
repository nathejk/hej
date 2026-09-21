package patroltrack

import (
	"reflect"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/trackpoint"
)

// The merge (task 340). The tests that matter most are the two about what the output must *not* contain:
// no interleaving between people, and no bridge across a gap.

var start = time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC).UnixMilli()

// ms returns a timestamp n seconds into the night.
func ms(sec int) int64 { return start + int64(sec)*1000 }

func p(lat, lng float64, sec int) trackpoint.Point {
	return trackpoint.Point{TS: ms(sec), Lat: lat, Lng: lng}
}

// A straight walk north, sampled every 30 s. Simplification should reduce it to its endpoints, because a
// straight line is an honest description of a straight walk.
func straightWalk(lng float64, fromSec, samples int) []trackpoint.Point {
	out := make([]trackpoint.Point, 0, samples)
	for i := 0; i < samples; i++ {
		out = append(out, p(55.700+float64(i)*0.0005, lng, fromSec+i*30))
	}
	return out
}

func TestMergeEmptyInput(t *testing.T) {
	for name, groups := range map[string][][]trackpoint.Point{
		"nil":            nil,
		"no groups":      {},
		"an empty group": {{}},
	} {
		track := Merge(groups)
		if !track.IsEmpty() {
			t.Errorf("%s: want an empty track, got %d segments", name, len(track.Segments))
		}
		if track.Recorders != 0 {
			t.Errorf("%s: want 0 recorders, got %d", name, track.Recorders)
		}
	}
}

func TestMergeDrawsOneSegmentForAContinuousWalk(t *testing.T) {
	track := Merge([][]trackpoint.Point{straightWalk(12.200, 0, 20)})

	if len(track.Segments) != 1 {
		t.Fatalf("want 1 segment, got %d", len(track.Segments))
	}
	if track.Recorders != 1 {
		t.Errorf("want 1 recorder, got %d", track.Recorders)
	}
	if track.SourcePoints != 20 {
		t.Errorf("want 20 source points, got %d", track.SourcePoints)
	}
	// A straight line simplifies to its endpoints.
	if got := len(track.Segments[0].Points); got != 2 {
		t.Errorf("a straight walk should simplify to 2 points, got %d", got)
	}
}

// **The zig-zag artefact this package exists to avoid.** Two members walking ten metres apart, sampling at
// alternating moments. Interleaving them by time produces a line that jumps between the two a hundred
// times; the union of their segments does not.
func TestTwoMembersNeverInterleave(t *testing.T) {
	// Member A on one side of the path, B ten metres east, sampling 15 s out of phase.
	var a, b []trackpoint.Point
	for i := 0; i < 20; i++ {
		lat := 55.700 + float64(i)*0.0005
		a = append(a, p(lat, 12.2000, i*30))
		b = append(b, p(lat, 12.20016, i*30+15)) // ~10 m east
	}

	track := Merge([][]trackpoint.Point{a, b})

	if len(track.Segments) != 2 {
		t.Fatalf("want one segment per member, got %d", len(track.Segments))
	}
	// Within each segment the longitude must be constant — every point from one person. An interleaved
	// merge would alternate between the two longitudes.
	for i, seg := range track.Segments {
		first := seg.Points[0].Lng
		for _, pt := range seg.Points {
			if pt.Lng != first {
				t.Errorf("segment %d mixes points from two members (lng %f then %f): this is the "+
					"zig-zag artefact", i, first, pt.Lng)
				break
			}
		}
	}
	if track.Recorders != 2 {
		t.Errorf("want 2 recorders, got %d", track.Recorders)
	}
}

// **Gaps are gaps.** A pocket-borne hour must not be bridged by a confident straight line.
func TestASilenceBreaksTheSegment(t *testing.T) {
	// Twenty minutes of walking, an hour in a pocket, then twenty more.
	before := straightWalk(12.200, 0, 10)
	after := straightWalk(12.300, 3600, 10)

	track := Merge([][]trackpoint.Point{append(before, after...)})

	if len(track.Segments) != 2 {
		t.Fatalf("an hour of silence must break the segment: got %d segments", len(track.Segments))
	}
	// And no segment may span the gap.
	for i, seg := range track.Segments {
		first, last := seg.Points[0].Lng, seg.Points[len(seg.Points)-1].Lng
		if first != last {
			t.Errorf("segment %d spans the gap (lng %f to %f)", i, first, last)
		}
	}
}

// Ordinary jitter must not shatter a walk. The client samples at ~30 s and flushes every 2 minutes, so a
// gap of a minute or two is normal.
func TestOrdinaryJitterDoesNotBreakASegment(t *testing.T) {
	points := []trackpoint.Point{
		p(55.7000, 12.200, 0),
		p(55.7005, 12.200, 30),
		// A two-minute gap: a slow fix or a flush boundary, not a pocket.
		p(55.7010, 12.200, 150),
		p(55.7015, 12.200, 180),
	}
	track := Merge([][]trackpoint.Point{points})

	if len(track.Segments) != 1 {
		t.Errorf("a two-minute gap is jitter, not a break: got %d segments", len(track.Segments))
	}
}

// Exactly at the threshold is not over it, and just over it is.
func TestTheGapThresholdBoundary(t *testing.T) {
	exact := []trackpoint.Point{
		p(55.7000, 12.200, 0),
		p(55.7005, 12.200, int(GapThreshold.Seconds())),
	}
	if got := len(Merge([][]trackpoint.Point{exact}).Segments); got != 1 {
		t.Errorf("a gap of exactly the threshold should not break: got %d segments", got)
	}

	over := []trackpoint.Point{
		p(55.7000, 12.200, 0),
		p(55.7005, 12.200, int(GapThreshold.Seconds())+1),
	}
	// Two runs of one point each; both are dropped as un-drawable, so the track is empty rather than
	// carrying two stray dots.
	track := Merge([][]trackpoint.Point{over})
	if len(track.Segments) != 0 {
		t.Errorf("two single points either side of a gap draw nothing: got %d segments", len(track.Segments))
	}
}

// A single point is not a line, and must not be emitted as a one-point segment.
func TestSinglePointsAreNotDrawn(t *testing.T) {
	track := Merge([][]trackpoint.Point{{p(55.70, 12.20, 0)}})

	if !track.IsEmpty() {
		t.Errorf("one point is not a line: got %d segments", len(track.Segments))
	}
	// But the person did record something, and the count must say so — that is the difference between
	// "nobody recorded" and "somebody recorded almost nothing".
	if track.Recorders != 1 {
		t.Errorf("want 1 recorder, got %d", track.Recorders)
	}
	if track.SourcePoints != 1 {
		t.Errorf("want 1 source point, got %d", track.SourcePoints)
	}
}

// **Simplification must keep corners.** This is the difference between Douglas–Peucker and dropping every
// nth point: a route through a forest that loses its turns loses the features a patrol would recognise.
func TestSimplificationKeepsCornersAndDropsStraightRedundancy(t *testing.T) {
	// A right-angled turn: 10 points north, then 10 points east.
	var points []trackpoint.Point
	for i := 0; i < 10; i++ {
		points = append(points, p(55.700+float64(i)*0.0009, 12.200, i*30))
	}
	for i := 1; i <= 10; i++ {
		points = append(points, p(55.7081, 12.200+float64(i)*0.0016, (10+i)*30))
	}

	track := Merge([][]trackpoint.Point{points})
	if len(track.Segments) != 1 {
		t.Fatalf("want 1 segment, got %d", len(track.Segments))
	}
	kept := track.Segments[0].Points

	// Both straight runs collapse, but the corner between them survives: three points, not two.
	if len(kept) < 3 {
		t.Errorf("the corner was simplified away: %d points kept from %d", len(kept), len(points))
	}
	if len(kept) > 6 {
		t.Errorf("straight-line redundancy was not removed: %d points kept from %d", len(kept), len(points))
	}
}

// GPS wobble below the measurement error is noise, not detail. Preserving it to the metre would be
// dressing up the receiver's error as the patrol's path.
func TestSimplificationRemovesWobbleWithinAccuracy(t *testing.T) {
	// A straight walk with ±5 m of jitter — well inside the 10.5 m median accuracy task 082 measured.
	var points []trackpoint.Point
	for i := 0; i < 30; i++ {
		wobble := 0.00005 // ~5 m
		if i%2 == 0 {
			wobble = -wobble
		}
		points = append(points, p(55.700+float64(i)*0.0005, 12.200+wobble, i*30))
	}

	track := Merge([][]trackpoint.Point{points})
	kept := len(track.Segments[0].Points)
	if kept > 4 {
		t.Errorf("wobble inside the accuracy floor was preserved as detail: %d points kept from %d",
			kept, len(points))
	}
}

// **No point in the output may identify a member.** Enforced by the type, asserted here so a future field
// addition has to justify itself.
//
// The field list is pinned rather than counted, because task 342 legitimately added `TS` — needed by the
// distance estimate, which matches track points into scan legs by time. A timestamp is **not** a person, but
// it is close enough to one to be worth a rule: times may be used server-side and must not cross the wire,
// because precise times on a merged track let a reader infer that two overlapping segments belong to
// different people. `cmd/api`'s map-response test is what enforces the serialisation half.
func TestTheOutputCarriesNoPersonAnywhere(t *testing.T) {
	track := Merge([][]trackpoint.Point{
		straightWalk(12.200, 0, 10),
		straightWalk(12.300, 0, 10),
	})

	wantPointFields := map[string]bool{"Lat": true, "Lng": true, "TS": true}
	for _, f := range structFields(Point{}) {
		if !wantPointFields[f] {
			t.Errorf("patroltrack.Point gained %q. A route point is a coordinate and a time; anything else "+
				"— an accuracy, a person, a device — belongs in a deliberate decision, not in this struct", f)
		}
		delete(wantPointFields, f)
	}
	for f := range wantPointFields {
		t.Errorf("patroltrack.Point lost %q, which something depends on", f)
	}

	if got := structFields(Segment{}); len(got) != 1 {
		t.Errorf("patroltrack.Segment has %v; a segment is a list of points", got)
	}
	// And Recorders is a count, not a list: a count cannot be turned back into who declined.
	if track.Recorders != 2 {
		t.Errorf("want 2 recorders, got %d", track.Recorders)
	}
}

// Two members standing together legitimately record the same place at the same moment. Collapsing that
// would assert they were one person.
func TestIdenticalPointsFromTwoMembersAreBothKept(t *testing.T) {
	together := []trackpoint.Point{
		p(55.700, 12.200, 0),
		p(55.705, 12.200, 300),
	}
	// The same walk, recorded by two phones at the same instants.
	track := Merge([][]trackpoint.Point{together, together})

	if len(track.Segments) != 2 {
		t.Errorf("two members recording the same walk are two segments, got %d", len(track.Segments))
	}
	if track.Recorders != 2 {
		t.Errorf("want 2 recorders, got %d", track.Recorders)
	}
}

// The counts must describe the track honestly, since the page uses them to say what it covers.
func TestCountsDescribeWhatWentInAndWhatCameOut(t *testing.T) {
	track := Merge([][]trackpoint.Point{straightWalk(12.200, 0, 40)})

	if track.SourcePoints != 40 {
		t.Errorf("want 40 source points, got %d", track.SourcePoints)
	}
	if track.Points >= track.SourcePoints {
		t.Errorf("simplification should have reduced 40 points, got %d", track.Points)
	}
	if track.Points != len(track.Segments[0].Points) {
		t.Errorf("Points (%d) should total the segments' points (%d)",
			track.Points, len(track.Segments[0].Points))
	}
}

func TestPerpendicularDistanceIsInMetres(t *testing.T) {
	// A point ~11 m north of a line running east along 55.7.
	a := trackpoint.Point{Lat: 55.700, Lng: 12.200}
	b := trackpoint.Point{Lat: 55.700, Lng: 12.300}
	off := trackpoint.Point{Lat: 55.7001, Lng: 12.250}

	got := perpendicularMetres(off, a, b)
	if got < 10 || got > 13 {
		t.Errorf("want ~11 m, got %.2f", got)
	}

	// On the line is zero.
	if got := perpendicularMetres(trackpoint.Point{Lat: 55.700, Lng: 12.250}, a, b); got > 0.01 {
		t.Errorf("a point on the line should be 0 m away, got %.4f", got)
	}
}

// A degenerate line (a and b the same place) must not divide by zero.
func TestPerpendicularDistanceToAPoint(t *testing.T) {
	a := trackpoint.Point{Lat: 55.700, Lng: 12.200}
	got := perpendicularMetres(trackpoint.Point{Lat: 55.7005, Lng: 12.200}, a, a)
	if got < 50 || got > 60 {
		t.Errorf("want ~55 m, got %.2f", got)
	}
}

// structFields is a tiny reflection helper so the "no person" assertion reads as a claim about the type.
func structFields(v any) []string {
	return fieldNames(v)
}

func fieldNames(v any) []string {
	t := reflect.TypeOf(v)
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		out = append(out, t.Field(i).Name)
	}
	return out
}

// The per-point outlier filter (PRD 011 §0b.6, task 349).
//
// The rule these tests pin is the one the maintainer asked for: **drop the coordinate, not the leg**. A
// filter that removed the stretch around a bad fix would throw away ground the patrol really covered, and
// that is the failure the old speed-based leg filter had.

// pa is p with a reported accuracy.
func pa(lat, lng float64, sec int, accuracy float64) trackpoint.Point {
	pt := p(lat, lng, sec)
	pt.Accuracy = accuracy
	return pt
}

func TestAWildlyInaccurateFixIsDropped(t *testing.T) {
	// A straight walk with one cell-tower fix in the middle, a kilometre off the line.
	points := []trackpoint.Point{
		pa(55.700, 12.200, 0, 8),
		pa(55.7005, 12.200, 30, 12),
		// 11.8 km accuracy — the worst value in the 2026 dev telemetry, and by its own admission not a
		// position.
		pa(55.710, 12.215, 60, 11820),
		pa(55.7015, 12.200, 90, 9),
		pa(55.7020, 12.200, 120, 10),
	}

	track := Merge([][]trackpoint.Point{points})

	if track.DroppedPoints != 1 {
		t.Errorf("DroppedPoints = %d, want 1", track.DroppedPoints)
	}
	// **One segment, not two.** The bad point is removed before the gaps are found, so its neighbours
	// join up as the single walk they were.
	if len(track.Segments) != 1 {
		t.Fatalf("want 1 segment, got %d — the filter broke the walk instead of mending it", len(track.Segments))
	}
	for _, pt := range track.Segments[0].Points {
		if pt.Lng > 12.21 {
			t.Errorf("the discarded fix is still on the track: %v", pt)
		}
	}
}

// Accuracy 0 means **unknown, not perfect**. Dropping those would remove whole patrols whose phones report
// no accuracy at all, which is a much worse failure than keeping a few imprecise points.
func TestUnknownAccuracyIsKept(t *testing.T) {
	track := Merge([][]trackpoint.Point{straightWalk(12.200, 0, 10)})

	if track.DroppedPoints != 0 {
		t.Errorf("DroppedPoints = %d; points with no reported accuracy must be kept", track.DroppedPoints)
	}
	if len(track.Segments) != 1 {
		t.Fatalf("want 1 segment, got %d", len(track.Segments))
	}
}

// **A spike costs one point, and the walk either side of it survives.** This is the acceptance criterion
// task 349 is built around: the old filter would have dropped both legs.
func TestASpikeCostsOnePointNotTwoSegments(t *testing.T) {
	// A walk with one fix that jumps ~40 km away and back within 30 s each way — hundreds of km/h in
	// both directions, which nothing physical does.
	points := []trackpoint.Point{
		p(55.700, 12.200, 0),
		p(55.7005, 12.200, 30),
		p(56.060, 12.200, 60), // the spike
		p(55.7015, 12.200, 90),
		p(55.7020, 12.200, 120),
	}

	track := Merge([][]trackpoint.Point{points})

	if track.DroppedPoints != 1 {
		t.Errorf("DroppedPoints = %d, want exactly the spike", track.DroppedPoints)
	}
	if len(track.Segments) != 1 {
		t.Fatalf("want 1 segment, got %d", len(track.Segments))
	}
	for _, pt := range track.Segments[0].Points {
		if pt.Lat > 55.8 {
			t.Errorf("the spike is still on the track: %v", pt)
		}
	}
}

// **One-way fast is not a spike.** A phone that moves fast and stays there is a patrol in a car (or the
// first fix after a long silence) — PRD 011 §0b.6 handles that by race status, not here. Deleting it as an
// error would remove a real journey *and* make a car look like a problem we had solved.
func TestAOneWayFastStepIsKept(t *testing.T) {
	points := []trackpoint.Point{
		p(55.700, 12.200, 0),
		p(55.7005, 12.200, 30),
		// 40 km in 30 s: absurd as a step, but the track *stays* up here afterwards, so it is movement
		// (or a gap), not a fix that teleported and came back.
		p(56.060, 12.200, 60),
		p(56.0605, 12.200, 90),
		p(56.0610, 12.200, 120),
	}

	track := Merge([][]trackpoint.Point{points})

	if track.DroppedPoints != 0 {
		t.Errorf("DroppedPoints = %d; a one-way fast step is movement, not a spike", track.DroppedPoints)
	}
}

// Every point unusable is an empty track rather than a one-point segment or a panic. A patrol whose only
// recorder had a broken GPS reads as "no route", which is a state the page already handles.
func TestAllPointsUnusable(t *testing.T) {
	points := []trackpoint.Point{
		pa(55.700, 12.200, 0, 5000),
		pa(55.701, 12.201, 30, 5000),
	}

	track := Merge([][]trackpoint.Point{points})

	if !track.IsEmpty() {
		t.Errorf("want an empty track, got %d segments", len(track.Segments))
	}
	if track.DroppedPoints != 2 {
		t.Errorf("DroppedPoints = %d, want 2", track.DroppedPoints)
	}
	// The member still counts as a recorder: they *did* record, and the count is what tells the page
	// "somebody tried" apart from "nobody recorded".
	if track.Recorders != 1 {
		t.Errorf("Recorders = %d, want 1", track.Recorders)
	}
}

// Two bad fixes in a row must not shelter each other: the comparison is against the last **kept** point, so
// the second one is judged against the walk rather than against its equally-bad predecessor.
func TestTwoConsecutiveSpikesDoNotShelterEachOther(t *testing.T) {
	points := []trackpoint.Point{
		p(55.700, 12.200, 0),
		p(55.7005, 12.200, 30),
		p(56.060, 12.200, 60),  // spike
		p(56.0601, 12.200, 90), // and another, right next to the first
		p(55.7015, 12.200, 120),
		p(55.7020, 12.200, 150),
	}

	track := Merge([][]trackpoint.Point{points})

	if track.DroppedPoints != 2 {
		t.Errorf("DroppedPoints = %d, want both spikes", track.DroppedPoints)
	}
	for _, seg := range track.Segments {
		for _, pt := range seg.Points {
			if pt.Lat > 55.8 {
				t.Errorf("a spike survived: %v", pt)
			}
		}
	}
}
