// Package patroltrack turns a patrol's members' recorded positions into one drawable route
// (PRD 011 §6, §0b.1; task 340).
//
// # One track, no attribution, and that is a requirement rather than a simplification
//
// The public surface's whole privacy claim is that it names no person (PRD 011 §0b.1). An attributed track
// would break it outright, and it would disclose two things nobody agreed to publish:
//
//   - **who declined location** — visibly absent from a map the rest of the patrol is on;
//   - **who left the group** — visibly somewhere else.
//
// So the output is segments with no person on them, and there is no way to ask which member a segment came
// from. The type is the enforcement; `Segment` has nowhere to put an id.
//
// # The merge is a union of segments, never an interleaving of points
//
// The tempting implementation is to sort every member's points into one time-ordered list and draw it. That
// produces a **zig-zag artefact**: two members walking ten metres apart, sampling at slightly different
// moments, yield a line that jumps between them a hundred times a kilometre. It looks like a drunken walk
// and it is purely an artefact of interleaving.
//
// So each member's points are simplified and broken independently, and the results are concatenated. The
// drawn route is a multi-segment polyline in one colour, which reads as "the patrol went this way" without
// claiming any single person did.
//
// # Gaps are gaps
//
// Task 082 measured the recorded track at **2% of a 22-hour day**, with one gap per backgrounding matching
// it almost to the second. Joining two points either side of a two-hour hole draws a confident straight
// line through terrain nobody walked. So a segment breaks when the time between consecutive points exceeds
// `GapThreshold`, and the gaps are left visible — they are information, and smoothing them away is the one
// presentation choice PRD 011 §0a says must not be made.
//
// # Why this is a package
//
// Everything here is a pure function of points. The merge rule, the gap rule and the simplification are
// the parts worth getting right, and they are worth testing without a database — the same reasoning behind
// `internal/distance` and `internal/imaging`.
package patroltrack

import (
	"math"
	"time"

	"nathejk.dk/nathejk/table/trackpoint"
)

// GapThreshold is how long a silence has to be before a segment breaks.
//
// # Why five minutes
//
// The client samples at ~30 s and flushes every 2 minutes (task 083). So a gap of a minute or two is
// ordinary jitter — a slow fix, a flush boundary — and breaking on it would shatter a continuous walk into
// hundreds of fragments that draw identically to one line but cost more to send.
//
// Five minutes is ten missed samples. At a patrol's 3–4 km/h that is 250–350 m of unrecorded ground, which
// is about the point at which a straight line between the two ends stops being a fair description of the
// route and starts being an invention.
//
// It is a **multiple of the sampling interval**, as PRD 011 §6 requires — ten of them — rather than a round
// number chosen for looking tidy.
const GapThreshold = 5 * time.Minute

// SimplifyMetres is the tolerance the Douglas–Peucker simplification runs at.
//
// # Why 15 metres, against a 10.5 m median accuracy
//
// Task 082 measured 10.5 m median accuracy on an iPhone and 35 m on a Wi-Fi-only iPad. A tolerance *below*
// the measurement error would be preserving noise to the nearest metre and calling it detail: the wobble
// it keeps is the GPS's, not the patrol's.
//
// 15 m is just above the phone median, so it removes the wobble while keeping every real corner — a turn
// at a junction moves the line by tens of metres, not by ten. It is deliberately well below the iPad's
// 35 m: simplifying to *that* would start rounding off real geometry for everybody in order to flatter the
// worst device.
const SimplifyMetres = 15.0

// MaxAccuracyMetres is the reported accuracy beyond which a point is not used.
//
// # Why a point is dropped for its accuracy at all
//
// The event runs on whatever phones the patrols brought (PRD 011 §0b.6). A fix with a 2 km radius is not
// a position, it is a cell tower, and it does the most damage in the least visible way: it lands in the
// middle of a walk, inflates the two segments either side of it, and draws a spike across a lake.
//
// # Why 250
//
// Measured rather than chosen (task 349). In the 2026 dev telemetry — 648 points, 63 people — the
// distribution is: 542 points at or under 20 m, 46 under 35 m, 53 under 100 m, then **nothing at all
// between 100 m and 500 m**, and 7 above it including one at 11.8 km. That gap is what a threshold
// should sit in, and 250 m sits in the middle of it.
//
// It is deliberately far above task 082's measurements (10.5 m median on a phone, 35 m on a Wi-Fi-only
// iPad) so that no real device is excluded for being ordinary. The bar is "obviously wrong", not "less
// accurate than the best phone" — a 100 m fix in a forest is a fix.
//
// Accuracy 0 means *unknown*, not perfect, and is kept: some clients report nothing, and discarding
// every point from those devices would silently remove whole patrols rather than bad fixes.
const MaxAccuracyMetres = 250.0

// SpikeKmh is the implied speed at which a single point is treated as a GPS spike rather than as travel.
//
// # Why this is not a walking-pace threshold
//
// Because it is not asking "was this walked?". `internal/distance` asks that. This asks "is this point
// physically possible at all?", and it has to stay clear of every legitimate way a patrol's phone moves:
// a patrol being driven is **not** a spike (that is handled by status — PRD 011 §0b.6), so the bar has to
// be above a car. 200 km/h is above a car and a bus, and far below the hundreds or thousands implied by a
// fix that jumps to another county and back.
//
// # And why speed alone is not enough
//
// A point is only dropped when the track **comes back** — see returnsSoon. Fast away and staying away is a
// phone that moved; fast away and back is a receiver failing. Conflating the two is how a car journey gets
// quietly deleted as an error, or a genuine error kept as a journey.
const SpikeKmh = 200.0

// SpikeLookahead is how many later points may be inspected when deciding whether a track *returned*.
//
// A GPS excursion is short — one fix, occasionally two or three while the receiver recovers. A journey is
// not. So a small window is enough to tell them apart, and keeping it small is what makes the rule cheap
// (the scan is O(n·SpikeLookahead), over every point of every member of every patrol) and conservative: a
// long absurd stretch is left alone rather than deleted wholesale, which is the safe direction, because
// that case is movement and belongs to the race-status rule instead.
const SpikeLookahead = 5

// Point is one position on a drawn route.
//
// # Why there is a timestamp here and none in the JSON
//
// The map draws a line and has no use for a time, so an earlier version of this type carried only a
// coordinate. That turned out to break the distance estimate: `distance.Compute` matches track points into
// scan legs **by time**, so a timeless point can raise nothing (task 341 shipped with the track
// contributing zero, and said so).
//
// So the time is here, and the **map response does not serialise it** (task 342). That is not squeamishness:
// precise times on a merged track would let a reader infer that two overlapping segments must belong to
// different people, and from there how the patrol split up. `Recorders` already answers "how many
// recorded?" as a count, which is the most that question should yield.
//
// The rule, for whoever adds the next consumer: **times may be used server-side and must not cross the
// wire.** Task 337's route walk is what enforces it.
type Point struct {
	Lat, Lng float64

	// TS is epoch milliseconds. Server-side only — see above.
	TS int64
}

// Segment is one unbroken stretch of recorded route.
//
// A segment means "these points were recorded closely enough in time that a line between them is a fair
// description". A break between segments means "we do not know what happened here" — which the map must
// render as a gap rather than bridging.
type Segment struct {
	Points []Point
}

// Track is a patrol's whole drawable route.
type Track struct {
	// Segments are the unbroken stretches, in no meaningful order.
	//
	// Unordered on purpose: they come from different people and ordering them would imply a sequence the
	// data does not support. The map draws them all; nothing reads them as a route in order.
	Segments []Segment

	// Points is how many positions survived simplification, across every segment. For the map's own sake
	// and for the page's honesty sentence.
	Points int

	// DroppedPoints is how many points were discarded as unusable before anything was drawn (task 349).
	//
	// A diagnostic, never shown to a visitor: "we threw away 3 of your positions" is not a sentence a
	// public page should venture. It is here because a filter with no counter is a filter nobody can tell
	// is working — and because a patrol whose track looks short can be explained by it.
	DroppedPoints int

	// SourcePoints is how many points went in, before dedup within a person and simplification.
	//
	// Kept so the page can say what the track covers rather than implying it is a route. A track of 180
	// points from 4,000 recorded is a different claim from 180 from 190.
	SourcePoints int

	// Recorders is how many of the patrol's members contributed any point at all.
	//
	// **A count, never a list.** It is the one number that says whether an empty-looking map means "nobody
	// recorded" or "one person recorded a little", which is the difference between a broken feature and a
	// quiet night. A count cannot be turned back into who declined; a list could.
	Recorders int
}

// IsEmpty reports whether there is anything to draw.
func (t Track) IsEmpty() bool { return len(t.Segments) == 0 }

// Merge turns per-person point groups into one unattributed track.
//
// `groups` is one slice per person, each already in time order — the shape `trackpoint.Queries.ByPeople`
// returns, chosen so the correct merge is the easy one.
//
// Points within a person are assumed deduplicated by the projection's primary key (task 083's contract, as
// a constraint). Merge does not re-deduplicate across people, and must not: two members standing together
// legitimately recorded the same place at the same moment, and collapsing that would be asserting they were
// one person.
func Merge(groups [][]trackpoint.Point) Track {
	var out Track

	for _, points := range groups {
		if len(points) == 0 {
			continue
		}
		out.Recorders++
		out.SourcePoints += len(points)

		// **Unusable points go first, before the gaps are found.** Order matters: a wild fix sitting
		// inside a run would otherwise break the run around itself, and a spike removed afterwards would
		// leave the two halves as separate segments with a hole where the bad point was (PRD 011 §0b.6,
		// task 349). Dropping first lets the neighbours join up as the single walk they were.
		//
		// This is also why the filter lives here rather than in `internal/distance`: both the map and the
		// distance read this merge, and a point dropped in one place but not the other would draw a route
		// that disagrees with the number printed above it.
		usable, dropped := dropUnusable(points)
		out.DroppedPoints += dropped
		if len(usable) == 0 {
			continue
		}

		for _, run := range breakOnGaps(usable) {
			simplified := simplify(run, SimplifyMetres)
			if len(simplified) < 2 {
				// A single point is not a line. Dropped rather than emitted as a one-point segment,
				// which every map library draws as nothing or as a stray dot depending on its mood.
				continue
			}
			out.Segments = append(out.Segments, Segment{Points: toPoints(simplified)})
			out.Points += len(simplified)
		}
	}
	return out
}

// dropUnusable removes points that are not positions, one point at a time.
//
// # Why the unit of rejection is a single coordinate
//
// Task 339 filtered whole **legs** by speed, on the theory that a fast stretch was a vehicle transfer.
// PRD 011 §0b.6 corrected that: there are no transfer sections, and the thing that actually goes wrong is
// a mixed fleet of GPS units producing the occasional nonsense fix. One bad point corrupts the two
// segments either side of it — so removing the point fixes both, while removing a leg throws away a
// stretch the patrol really walked.
//
// # Two rules, in order
//
//  1. **Accuracy.** A fix whose own reported radius exceeds MaxAccuracyMetres is not a position. Cheap,
//     independent of neighbours, and the only rule with a *self-reported* justification — the device is
//     telling us it does not know where it is.
//  2. **Spikes.** A point that is implausibly fast away from the previous kept point, where the track then
//     **comes back** to somewhere plausible within the next few fixes. Judged against the previous *kept*
//     point rather than the previous raw one, so two consecutive bad fixes cannot shelter each other by
//     making the step between them look small.
//
// Returns the survivors and how many were dropped. The input is not modified.
func dropUnusable(points []trackpoint.Point) ([]trackpoint.Point, int) {
	kept := make([]trackpoint.Point, 0, len(points))
	dropped := 0

	for i, p := range points {
		if p.Accuracy > MaxAccuracyMetres {
			dropped++
			continue
		}
		if len(kept) > 0 {
			last := kept[len(kept)-1]
			if impliedKmh(last, p) > SpikeKmh && returnsSoon(points, i+1, last) {
				dropped++
				continue
			}
		}
		kept = append(kept, p)
	}
	return kept, dropped
}

// returnsSoon reports whether the track comes back to somewhere plausible shortly after `from`.
//
// **This is the test that separates an error from a journey**, and it is the whole reason the rule is not
// just "too fast". A fix that jumps away and comes back is a receiver failing. A position that jumps away
// and *stays* there is a phone that moved — a car, a train, or the first fix after a long silence — which
// PRD 011 §0b.6 handles by the member's race status instead. Deleting that as an error would remove a real
// journey, and would also make a car look like a data problem we had already dealt with.
//
// "Plausible" is measured from the same anchor the suspect point was measured from, so a long gap makes the
// implied speed small and nothing is dropped — the desired outcome, since after a two-hour silence we know
// nothing about what happened in between.
func returnsSoon(points []trackpoint.Point, from int, anchor trackpoint.Point) bool {
	for i := from; i < len(points) && i < from+SpikeLookahead; i++ {
		if points[i].Accuracy > MaxAccuracyMetres {
			// Not a witness to anything: this point is being dropped on its own account.
			continue
		}
		if impliedKmh(anchor, points[i]) <= SpikeKmh {
			return true
		}
	}
	return false
}

// impliedKmh is the speed a straight line between two fixes implies, or 0 when it cannot be computed.
//
// Flat-earth distance at 55°N, like perpendicularMetres and for the same reason: this decides whether a
// number is in the hundreds, and the projection error is metres.
func impliedKmh(a, b trackpoint.Point) float64 {
	hours := float64(b.TS-a.TS) / float64(time.Hour/time.Millisecond)
	if hours <= 0 {
		return 0
	}

	const metresPerDegreeLat = 111_320.0
	metresPerDegreeLng := metresPerDegreeLat * math.Cos(a.Lat*math.Pi/180)
	dx := (b.Lng - a.Lng) * metresPerDegreeLng
	dy := (b.Lat - a.Lat) * metresPerDegreeLat

	return math.Hypot(dx, dy) / 1000 / hours
}

// breakOnGaps splits one person's points wherever the recording stopped for too long.
func breakOnGaps(points []trackpoint.Point) [][]trackpoint.Point {
	var runs [][]trackpoint.Point
	current := []trackpoint.Point{points[0]}

	for i := 1; i < len(points); i++ {
		gap := time.Duration(points[i].TS-points[i-1].TS) * time.Millisecond
		if gap > GapThreshold {
			runs = append(runs, current)
			current = []trackpoint.Point{points[i]}
			continue
		}
		current = append(current, points[i])
	}
	return append(runs, current)
}

// simplify runs Douglas–Peucker over a run of points.
//
// # Why Douglas–Peucker and not "every nth point"
//
// Because decimation is wrong in the way that matters. Dropping every second point removes a sharp turn as
// readily as a straight stretch, so a route through a forest loses its corners — the features a patrol
// would recognise — while keeping redundant points along a road. Douglas–Peucker removes a point only when
// the line without it stays within the tolerance, so it deletes straight-line redundancy and preserves
// geometry by definition.
//
// # Recursion depth
//
// Recursive, and bounded in practice: the worst case is a pathological zig-zag at exactly the tolerance,
// and the input is one person's run between gaps — a few hundred points. A non-recursive version would be
// an explicit stack saying the same thing less clearly.
func simplify(points []trackpoint.Point, toleranceMetres float64) []trackpoint.Point {
	if len(points) < 3 {
		return points
	}

	first, last := points[0], points[len(points)-1]

	maxDist := -1.0
	maxIdx := 0
	for i := 1; i < len(points)-1; i++ {
		if d := perpendicularMetres(points[i], first, last); d > maxDist {
			maxDist, maxIdx = d, i
		}
	}

	if maxDist <= toleranceMetres {
		// Every intermediate point is within tolerance of the straight line, so the straight line is an
		// honest description of this run.
		return []trackpoint.Point{first, last}
	}

	left := simplify(points[:maxIdx+1], toleranceMetres)
	right := simplify(points[maxIdx:], toleranceMetres)
	// The split point is in both halves; drop it once so it is not duplicated in the output.
	return append(left[:len(left)-1], right...)
}

// perpendicularMetres is the distance from p to the line through a and b, in metres.
//
// # Why a local flat projection rather than spherical geometry
//
// Because the distances involved are tens of metres over runs of a few kilometres, at 55°N. Projecting
// longitude by cos(lat) and treating the result as a plane is accurate to far better than a metre at that
// scale — well inside a 15 m tolerance — and it avoids great-circle trigonometry in the inner loop of a
// recursion that runs over every point of every member of every patrol.
//
// The race-area buffer makes the same simplification for the same reason, and `internal/distance` does
// *not*, because there the error accumulates over a sum of long legs.
func perpendicularMetres(p, a, b trackpoint.Point) float64 {
	const metresPerDegreeLat = 111_320.0
	metresPerDegreeLng := metresPerDegreeLat * math.Cos(a.Lat*math.Pi/180)

	px := (p.Lng - a.Lng) * metresPerDegreeLng
	py := (p.Lat - a.Lat) * metresPerDegreeLat
	bx := (b.Lng - a.Lng) * metresPerDegreeLng
	by := (b.Lat - a.Lat) * metresPerDegreeLat

	lenSq := bx*bx + by*by
	if lenSq == 0 {
		// a and b are the same place, so "distance from the line" is distance from the point.
		return math.Hypot(px, py)
	}
	// The cross product's magnitude over the base length is the height of the triangle.
	return math.Abs(px*by-py*bx) / math.Sqrt(lenSq)
}

func toPoints(in []trackpoint.Point) []Point {
	out := make([]Point, 0, len(in))
	for _, p := range in {
		out = append(out, Point{Lat: p.Lat, Lng: p.Lng, TS: p.TS})
	}
	return out
}
