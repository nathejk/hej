// Package distance estimates how far a patrol walked (PRD 011 §6, task 339).
//
// # The obvious implementation is wrong by an order of magnitude
//
// The natural move is to sum the patrol's recorded track: add up the distance between consecutive
// position samples. That would produce a number roughly **fifty times too small**, and it would look
// entirely plausible.
//
// Task 082 measured the recorded track at **2% of a 22-hour day**, because a web app does not run
// while backgrounded — the phone is in a pocket for almost the whole night, and the track is a dotted
// record of the minutes the app was open. A patrol that walked 30 km would be told it walked 600 m,
// and nobody reviewing the code would see anything obviously amiss.
//
// # So the base is the scans, not the track
//
// A scan is a place the patrol demonstrably stood, with a time. Between two consecutive scans the
// patrol travelled *at least* the straight-line distance, because a straight line is the shortest path
// between two points. Summing those gives a **true lower bound**: not what they walked, but a number
// they certainly walked more than.
//
// The track then has exactly one job: where it covers a leg, the measured path is usually longer than
// the straight line, so it raises that leg. It can only ever raise. A leg where the track says less is
// a leg where the track missed part of the walk, which is the normal case rather than evidence.
//
// # Patrols are transported, and 2025 proved it matters
//
// The floor argument above has a hole, and it was found by checking the figures against the 2025 event
// rather than by reasoning (task 339's sanity check — the numbers are in that task's log). Summing every
// leg gave a **median of 45.9 km and a maximum of 157.9 km**. Nobody walks 158 km in a night.
//
// Patrols are moved between sections of the course. A straight line between two scans either side of a bus
// ride is a real journey the patrol made, but it is not a distance they *walked* — so the sum was a floor
// on the wrong quantity, and the word "mindst" would have sat in front of a number that overstated the
// walk.
//
// So a leg whose implied speed exceeds a walking pace is **excluded**, not capped: capping would invent a
// walk of exactly the length the filter allows. With that, 2025's worst case falls from 157.9 km to 41 km
// and the median to 41.3 km — which is 3.4 km/h over a twelve-hour night, the pace the event actually runs
// at.
//
// **Amended by task 349:** the *explanation* above was wrong even though the filter was right. PRD 011
// §0b.6 records that there are no transfer sections — what produces a car-shaped track is a person who is
// no longer active in the race, which is now handled where that fact lives (`cmd/api/patroltrack.go`), and
// a bad GPS fix, which is now dropped one coordinate at a time (`internal/patroltrack`). This filter
// remains as a **backstop**, and 2025 is why it cannot be removed: that event has no telemetry, so neither
// new rule can fire there and speed is the only signal available. See MaxWalkingKmh.
//
// A residual remains and is documented there: a slow enough vehicle leg is indistinguishable from a fast
// walk.
//
// # And it is presented as a floor
//
// `mindst ~24 km`, never `23,47 km`. The second claims a measurement that does not exist. This is the
// one number on a page a twelve-year-old will screenshot and an adult will read, so it has to be a
// number neither of them can catch out.
//
// # Why this is a package and not a method
//
// Everything here is a pure function of positions and times. That is worth testing without a database,
// a request or a broker — the same reasoning that put `internal/track` and `internal/imaging` in their
// own packages.
package distance

import (
	"fmt"
	"math"
	"time"
)

// Point is one position with the time it was recorded.
type Point struct {
	Lat, Lng float64
	At       time.Time
}

// Scan is one registration the patrol collected.
//
// Lat/Lng are nil when the scan carried no position — a post can register a patrol by hand, and the
// scan projection stores coordinates as strings that may be absent or unparseable. Such a scan is a
// real scan that cannot contribute a leg, which is the distinction this type exists to carry.
type Scan struct {
	Lat, Lng *float64
	At       time.Time
}

// Estimate is what the page shows, and what it is allowed to claim.
type Estimate struct {
	// Km is the lower bound in kilometres, unrounded. Round for display with Label.
	Km float64

	// Legs is how many scan-to-scan journeys the figure is built from.
	//
	// Zero means no figure at all — one positioned scan makes no leg, and nor does none.
	Legs int

	// RaisedLegs is how many legs the track lengthened beyond their straight line.
	//
	// Kept because it is the honest measure of how much the track contributed: a figure built from 9
	// legs of which 0 were raised is a pure scan-to-scan floor, and saying so is better than implying
	// the route was measured.
	RaisedLegs int

	// VehicleLegs is how many legs were excluded as too fast to have been walked.
	//
	// Named for what it was thought to detect. Task 349 corrected that (PRD 011 §0b.6): it counts legs the
	// backstop filter rejected, which may be a car, a mis-stamped scan, or a scan attributed to the wrong
	// post — the filter cannot tell, and the name should not pretend it can.
	//
	// Not shown to a visitor — "we think you were driven between post 6 and post 7" is not a sentence a
	// public page should venture. Kept because it is the one diagnostic that says whether the filter is
	// doing anything, and because a patrol whose figure looks low can be explained by it.
	VehicleLegs int

	// UnplottableScans is how many scans carried no position and so could not anchor a leg.
	UnplottableScans int

	// Incomplete reports that a material share of the patrol's scans had no position, so the figure
	// understates by more than its own tolerance.
	//
	// The page must say so rather than presenting the number as whole. Under-reporting quietly is the
	// failure this field exists to prevent — a patrol told it walked 12 km when it walked 30 has been
	// misinformed more than if it had been told nothing.
	Incomplete bool
}

// HasFigure reports whether there is anything worth showing.
func (e Estimate) HasFigure() bool { return e.Legs > 0 && e.Km > 0 }

// MaxWalkingKmh is the implied speed above which a leg is taken to be too fast to have been walked.
//
// # What this is for now (amended, task 349)
//
// It was introduced as **vehicle-transfer detection**, and that description was wrong: PRD 011 §0b.6
// records the maintainer's correction that *there are no transfer sections*. What actually produces a
// car-shaped track is a person who is **no longer active in the race** — handled by their race status, in
// `cmd/api/patroltrack.go`, where their points stop counting from the moment they left — and what produces
// wild geometry is a bad GPS fix, handled per coordinate in `internal/patroltrack`.
//
// So this is a **backstop**, not the mechanism: it catches legs those two rules cannot see. And there are
// such legs, which is why it stays. The most important one: 2025 has **no telemetry at all** (the feature
// shipped afterwards), so for that event neither new rule can fire and every leg here is scan-to-scan. A
// patrol moved by car between two posts, with nobody's status changing, still shows up only as speed.
//
// # Why 7
//
// A patrol of twelve-year-olds with packs, at night, in a forest, averages 3–5 km/h. Seven is comfortably
// above any of them and well below a car, a bus or a train. It is chosen to be **hard to argue with in
// the direction that matters**: no patrol is excluded for walking briskly.
//
// The 2025 distribution supports it rather than merely permitting it: 3,045 legs fall under 7 km/h
// totalling 7,521 km, while 77 legs sit above 15 km/h — a clean separation with very little in between,
// which is what a threshold between two different activities should look like.
//
// # The residual, stated because it cannot be fixed here
//
// A leg slow enough to pass this filter — 20 km with a three-hour gap around it, which implies 6.7 km/h —
// cannot be told from a long walk using only positions and times. Task 349 measured 2025 again and found
// that this is where the whole overstating tail lives, and that it is better caught by **duration and
// distance together** than by speed: see MaxLegHours. What remains after both is a transfer of a few
// kilometres inside an ordinary gap, which is small and unknowable.
//
// Neither of task 349's other two rules helps here, and it is worth knowing why: they need telemetry and a
// status change, and 2025 has no telemetry at all.
//
// Until then the page must not claim more precision than "built from the posts you were scanned at", which
// is why the wording carries no route and no measurement.
const MaxWalkingKmh = 7.0

// MaxLegHours and MaxLegKm together reject a leg that is too long *and* too far to be one walk.
//
// # What this catches that the speed filter cannot
//
// Task 349 measured 2025 again and found the overstating tail is not made of fast legs — it is made of
// **slow, enormous** ones. The worst patrol's biggest contributions were 38.7 km over 13.6 hours (2.8 km/h)
// and 10.4 km over **143 hours**. Six days. Those are not journeys; they are two scans that do not belong to
// the same night, and a speed filter waves them through precisely because dividing by a huge number gives a
// walking pace.
//
// # Why both conditions, and not duration alone
//
// Because a long gap on its own is **normal**: patrols rest. The 2025 data has legs of six hours covering
// two kilometres — a patrol that slept, then walked to the next post. Rejecting those on duration would
// throw away real walking for no gain, since a leg that is long in time and short in distance contributes
// almost nothing anyway. What is not normal is a gap of hours that also spans a county.
//
// # The numbers, measured rather than chosen (2025, 168 patrols, 3,232 legs)
//
// 87% of legs are under two hours; only 150 exceed four. Applying "over 4 h *and* over 10 km" on top of the
// speed filter moves the event's figures from mean 44.8 km / max 103.5 km to **mean 38.1 km / max 63.6 km**,
// and the number of patrols credited with over 60 km from 26 to 1. Variants measured at the same time:
//
//	duration alone:  4 h → max 57.4   6 h → max 63.6   8 h → max 73.4   12 h → max 90.8
//	compound:        4 h + 8 km → max 57.4   4 h + 10 km → max 63.6   4 h + 15 km → max 64.8
//
// The compound rule at 10 km was chosen over the plain 6 h cut because both give the same maximum while the
// compound one keeps ~2 km per patrol of genuine walking that the duration cut discards.
//
// # The direction of the error
//
// Excluding a leg can only make the figure **lower**, which is the direction the word *mindst* permits.
// That is why a rule like this is allowed to be approximate: a real walk wrongly excluded understates a
// number already labelled as a floor, while a transfer wrongly included makes the label false.
const (
	MaxLegHours = 4.0
	MaxLegKm    = 10.0
)

// incompleteShare is the fraction of unplottable scans past which the figure is called incomplete.
//
// A third. Below that the scans that *are* positioned still bound the route reasonably — the patrol
// passed through those places in that order, so the missing ones mostly sit between points already
// counted. Above it, whole sections of the night are missing from the sum and the number stops being a
// useful floor and starts being a misleading one.
//
// The exact figure is a judgement, not a derivation. What matters is that there *is* a threshold and
// that crossing it changes what the page says.
const incompleteShare = 1.0 / 3.0

// Compute estimates the distance from a patrol's scans, raised by whatever the track covers.
//
// `scans` need not be sorted; they are ordered by time here, because the projection returns them
// newest-first and a sum over that order would be the same number for the wrong reason — the absolute
// distances happen to be symmetric, which would hide a real ordering bug from every test that only
// checked the total.
//
// `track` is every position sample for the patrol, merged and deduplicated by the caller (task 340).
// Nil is fine and is the common case: task 082 found most patrols record very little.
func Compute(scans []Scan, track []Point) Estimate {
	positioned, unplottable := splitByPosition(scans)

	est := Estimate{UnplottableScans: unplottable}
	if total := len(positioned) + unplottable; total > 0 {
		est.Incomplete = float64(unplottable)/float64(total) > incompleteShare
	}
	if len(positioned) < 2 {
		// One place is not a journey. Reported as no figure rather than as 0 km, which a template would
		// render as a claim that the patrol did not move.
		return est
	}

	sortByTime(positioned)
	sortPointsByTime(track)

	for i := 1; i < len(positioned); i++ {
		from, to := positioned[i-1], positioned[i]
		straight := Between(from.Lat, from.Lng, to.Lat, to.Lng)

		// Excluded rather than capped. A leg covered faster than anybody walks, or one spanning hours *and*
		// tens of kilometres, is not a walk between two posts — and capping it at a walking pace would
		// instead invent a walk of exactly the length the filter allows.
		if isVehicleLeg(straight, from.At, to.At) || isImplausibleLeg(straight, from.At, to.At) {
			est.VehicleLegs++
			continue
		}

		leg := straight
		if measured := alongTrack(track, from.At, to.At); measured > leg {
			leg = measured
			est.RaisedLegs++
		}

		est.Km += leg
		est.Legs++
	}
	return est
}

// isVehicleLeg reports whether a leg was covered too fast to have been walked.
//
// # Zero and negative durations are not vehicles
//
// Two scans in the same second imply an infinite speed, and treating that as a vehicle would drop a leg
// over a clock artefact. They are almost always a post scanning a patrol twice, where the distance is
// near zero anyway — so the leg is kept and contributes approximately nothing, which is the right answer
// for the right reason.
func isVehicleLeg(km float64, from, to time.Time) bool {
	hours := to.Sub(from).Hours()
	if hours <= 0 {
		return false
	}
	return km/hours > MaxWalkingKmh
}

// isImplausibleLeg reports whether a leg lasted too long *and* covered too far to be one walk.
//
// The **and** is the rule: see MaxLegHours. A long gap alone is a rest, which is ordinary; a long gap that
// also crosses tens of kilometres is two scans from different journeys.
func isImplausibleLeg(km float64, from, to time.Time) bool {
	hours := to.Sub(from).Hours()
	if hours <= 0 {
		return false
	}
	return hours > MaxLegHours && km > MaxLegKm
}

// Between is the great-circle distance between two coordinates, in kilometres.
//
// Haversine rather than the flat-earth approximation the race-area buffer uses. That one is fine for a
// 3 km margin around a hull; this is summed over a dozen legs spanning tens of kilometres, and it is
// the number people will quote.
func Between(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371.0

	rad := func(deg float64) float64 { return deg * math.Pi / 180 }

	dLat := rad(lat2 - lat1)
	dLng := rad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}

// Leg is one journey between two positioned scans.
//
// Used by the public map to draw the legs the track does not cover — see UncoveredLegs.
type Leg struct {
	From, To Point
}

// UncoveredLegs returns the scan-to-scan journeys the track says nothing about.
//
// # Why this lives next to the estimate rather than in the handler
//
// Because it must use **the same definition of "covered"** the figure uses: `alongTrack` — at least two
// track points inside the leg's time window. The public map draws an uncovered leg as a dotted line meaning
// "we know you went from here to there, not how", and the number above the map is built from the same test
// (Compute asks the track what it has to say about each leg, and can only use the answer where there is one).
// Two definitions would let the map dot a leg the distance had measured, which is a page nobody can explain.
//
// Note that covered is **not** the same as *raised*: a leg can have track inside it whose measured path is
// shorter than the straight line — a phone that woke for two samples — in which case `Compute` keeps the
// straight line and this function still calls the leg covered. That is the right split for both: the figure
// takes the longer of the two, while the map's question is only whether we recorded anything at all.
//
// And coverage is **by time, not by place**: a leg whose window contains track points is covered even if those
// points are somewhere else, which happens when one member recorded while the patrol walked elsewhere. The
// estimate has had that property since task 339 and it is inherited here deliberately rather than fixed in one
// place — a geographic test would be a different rule, and two rules is the thing this function exists to
// avoid.
//
// Legs the filters exclude (too fast, or too long *and* too far — see MaxWalkingKmh and MaxLegHours) are
// **included** here, deliberately. The map's claim is about what we recorded, not about what we counted: the
// patrol was registered at both ends, and we have no track in between. Leaving them out would draw nothing
// where the honest answer is "we do not know".
//
// Order is time order, and a leg needs two positioned scans — an unplottable scan anchors nothing, as on the
// page's list.
func UncoveredLegs(scans []Scan, track []Point) []Leg {
	positioned, _ := splitByPosition(scans)
	if len(positioned) < 2 {
		return nil
	}

	sortByTime(positioned)
	sortPointsByTime(track)

	var out []Leg
	for i := 1; i < len(positioned); i++ {
		from, to := positioned[i-1], positioned[i]
		if alongTrack(track, from.At, to.At) > 0 {
			continue
		}
		out = append(out, Leg{From: from, To: to})
	}
	return out
}

// alongTrack measures the recorded path between two instants, or 0 when the track does not cover it.
//
// # Why a partial window contributes nothing rather than something
//
// A leg is only raised when the track has at least two points inside it. A single point, or none, means
// the track says nothing about how that leg was walked — and a partial measurement would be *shorter*
// than the straight line more often than longer, which would make the `if measured > leg` test pass
// arbitrarily rather than meaningfully.
//
// The window is inclusive of both ends so a point recorded at the moment of a scan counts, which is
// exactly when a phone is out of a pocket.
func alongTrack(track []Point, from, to time.Time) float64 {
	if len(track) < 2 || !to.After(from) {
		return 0
	}

	var km float64
	var prev Point
	have := false
	for _, p := range track {
		if p.At.Before(from) {
			continue
		}
		if p.At.After(to) {
			break
		}
		if have {
			km += Between(prev.Lat, prev.Lng, p.Lat, p.Lng)
		}
		prev, have = p, true
	}
	return km
}

// Label renders the estimate the way the page says it, in Danish.
//
// # The wording and the rounding live here, together, and nowhere else
//
// Because they are one decision. `mindst ~24 km` is honest only if 24 is rounded in the direction the
// word "mindst" claims — which means rounding **down**, so the stated number is never larger than the
// computed floor. Round-half-up would occasionally print a number the patrol did not certainly walk,
// and the word in front of it would then be false.
//
// No decimals, ever. `23,5 km` implies a measurement to 100 m, from a figure built out of straight
// lines between posts.
//
// Below a kilometre there is no honest round number to give, so it says so in words instead of
// printing `mindst ~0 km`.
func Label(e Estimate) string {
	if !e.HasFigure() {
		return ""
	}

	km := int(math.Floor(e.Km))
	if km < 1 {
		return "under 1 km"
	}
	return fmt.Sprintf("mindst ~%d km", km)
}

// splitByPosition separates the scans that can anchor a leg from the ones that cannot.
func splitByPosition(scans []Scan) ([]Point, int) {
	positioned := make([]Point, 0, len(scans))
	unplottable := 0
	for _, s := range scans {
		if s.Lat == nil || s.Lng == nil {
			unplottable++
			continue
		}
		positioned = append(positioned, Point{Lat: *s.Lat, Lng: *s.Lng, At: s.At})
	}
	return positioned, unplottable
}

// sortByTime orders points oldest-first, in place.
//
// A hand-rolled insertion sort rather than `sort.Slice`: the input is a dozen scans and is usually
// already ordered (the projection returns them newest-first, so this is a reversal), and insertion sort
// is linear on nearly-sorted input. The real reason is that it is stable without saying so, which keeps
// two scans recorded in the same second in the order the projection returned them rather than an
// arbitrary one.
func sortByTime(points []Point) {
	for i := 1; i < len(points); i++ {
		for j := i; j > 0 && points[j].At.Before(points[j-1].At); j-- {
			points[j], points[j-1] = points[j-1], points[j]
		}
	}
}

func sortPointsByTime(points []Point) { sortByTime(points) }
