package patroltrack

import (
	"testing"
	"time"

	"nathejk.dk/nathejk/table/trackpoint"
)

// Real recorded points, from the `track_point` projection (task 340's verification).
//
// # Why this is pinned rather than generated
//
// Every other test in this package builds a tidy fixture: a straight walk, a clean right-angle, a gap of
// exactly an hour. Real telemetry is none of those. These nine points are one person's actual recording,
// and they are messy in the three ways that matter — irregular sampling (10 s, 48 s, 30 s, then 24
// minutes), sub-accuracy wobble of a few metres, and a trailing point stranded on its own past a gap.
//
// The person id is deliberately not here. It would serve no purpose and this package's whole point is that
// a route carries no person.
func realRecording() []trackpoint.Point {
	return []trackpoint.Point{
		{TS: 1789755157149, Lat: 55.83127, Lng: 12.05861},
		{TS: 1789755167208, Lat: 55.83126, Lng: 12.05864},
		{TS: 1789755215565, Lat: 55.83124, Lng: 12.05858},
		{TS: 1789755245565, Lat: 55.83127, Lng: 12.05859},
		// 24 minutes of silence: the phone went in a pocket.
		{TS: 1789756691786, Lat: 55.83117, Lng: 12.05861},
		{TS: 1789756745085, Lat: 55.83117, Lng: 12.05862},
		{TS: 1789756773163, Lat: 55.83111, Lng: 12.05862},
		{TS: 1789756948017, Lat: 55.83120, Lng: 12.05855},
		// 5.2 minutes later: just over the threshold, so this one is stranded alone.
		{TS: 1789757259566, Lat: 55.83118, Lng: 12.05864},
	}
}

func TestAgainstARealRecording(t *testing.T) {
	track := Merge([][]trackpoint.Point{realRecording()})

	// Two drawable runs, and the stranded ninth point dropped rather than emitted as a stray dot.
	if len(track.Segments) != 2 {
		t.Fatalf("want 2 segments, got %d", len(track.Segments))
	}
	if track.SourcePoints != 9 {
		t.Errorf("want 9 source points, got %d", track.SourcePoints)
	}
	if track.Recorders != 1 {
		t.Errorf("want 1 recorder, got %d", track.Recorders)
	}

	// Every point of this recording sits within ~20 m of every other — somebody at a desk, not walking —
	// so each run collapses to its endpoints. That is the right answer: there is no route here to draw.
	for i, seg := range track.Segments {
		if len(seg.Points) != 2 {
			t.Errorf("segment %d has %d points; a stationary run should collapse to its endpoints",
				i, len(seg.Points))
		}
	}
}

// The properties that must hold for *any* real recording, asserted against this one so the guarantees are
// checked against messy input rather than only against tidy fixtures.
func TestRealRecordingSatisfiesTheDrawingGuarantees(t *testing.T) {
	points := realRecording()
	track := Merge([][]trackpoint.Point{points})

	// 1. No segment may be undrawable.
	for i, seg := range track.Segments {
		if len(seg.Points) < 2 {
			t.Errorf("segment %d has %d points: a line needs two", i, len(seg.Points))
		}
	}

	// 2. Simplification never invents points.
	if track.Points > track.SourcePoints {
		t.Errorf("simplification produced %d points from %d", track.Points, track.SourcePoints)
	}

	// 3. **No segment may bridge a gap.** The output carries no timestamps, so this is checked against the
	//    input: every consecutive pair the merge kept must have come from a pair whose gap was inside the
	//    threshold. Verified by re-deriving the runs and confirming their count matches.
	runs := breakOnGaps(points)
	drawable := 0
	for _, run := range runs {
		if len(simplify(run, SimplifyMetres)) >= 2 {
			drawable++
		}
	}
	if drawable != len(track.Segments) {
		t.Errorf("the merge emitted %d segments but only %d runs are drawable", len(track.Segments), drawable)
	}

	// 4. Every kept point must be one of the recorded ones — simplification selects, it does not average.
	//    An averaged point would be a position nobody was at, drawn on a public map.
	recorded := map[[2]float64]bool{}
	for _, p := range points {
		recorded[[2]float64{p.Lat, p.Lng}] = true
	}
	for i, seg := range track.Segments {
		for j, p := range seg.Points {
			if !recorded[[2]float64{p.Lat, p.Lng}] {
				t.Errorf("segment %d point %d (%f,%f) was not recorded by anybody", i, j, p.Lat, p.Lng)
			}
		}
	}
}

// Dev data spans weeks rather than one night, which is a harsher input than a real event and worth keeping:
// a merge that coped only with a twelve-hour window would fall over on it.
func TestARecordingSpanningWeeksBreaksIntoSegmentsRatherThanOneLine(t *testing.T) {
	week := 7 * 24 * time.Hour
	points := []trackpoint.Point{
		{TS: 1789755157149, Lat: 55.8312, Lng: 12.0586},
		{TS: 1789755167208, Lat: 55.8313, Lng: 12.0587},
		{TS: 1789755157149 + week.Milliseconds(), Lat: 55.9000, Lng: 12.2000},
		{TS: 1789755167208 + week.Milliseconds(), Lat: 55.9001, Lng: 12.2001},
	}

	track := Merge([][]trackpoint.Point{points})

	if len(track.Segments) != 2 {
		t.Fatalf("a week-long gap must break the track, got %d segments", len(track.Segments))
	}
	// And crucially, no segment may contain points from both weeks — that line would cross 15 km of
	// Zealand the patrol never walked.
	for i, seg := range track.Segments {
		spread := seg.Points[len(seg.Points)-1].Lat - seg.Points[0].Lat
		if spread > 0.01 {
			t.Errorf("segment %d spans %f degrees of latitude: it bridged the gap", i, spread)
		}
	}
}
