package distance

// Leg is one step of the chain the public map draws dotted: two things we know, with nothing recorded between
// them. See UncoveredLegs.
type Leg struct {
	From, To Point
}

// UncoveredLegs returns the dotted lines the public map draws: every step between two things we know that we
// did not record.
//
// # The model: one chain of knowledge, in time order
//
// The map has two kinds of fact about a patrol, and both are facts about the patrol *as a whole*: a **scan**
// (it was here, at this instant) and a **recorded stretch** (somebody's phone drew this line, from here to
// there, between these instants). Lay all of them on one timeline and the page becomes a single chain: solid
// where a stretch was recorded, dotted between one fact and the next.
//
// A dotted leg therefore always says exactly one thing — *we know the patrol was at both ends and we did not
// record the way between* — whether it joins two scans, a scan to a stretch, or **two stretches to each other**.
//
// # Why segments and not points (the second attempt)
//
// This took two goes, and the failure is worth recording because it looked right and measured wrong.
//
// The first attempt walked the flat, time-ordered list of track points and dotted wherever consecutive points
// were more than `patroltrack.GapThreshold` apart. That is the rule `patroltrack.breakOnGaps` uses, so it seemed
// like the same question. It is not: `Merge` breaks segments **per recorder**, so with two phones recording at
// once the global point order alternates between them, every consecutive pair is close in time, and the rule
// concluded "this is a recording" for pairs that have **no line between them on the map at all**. Measured on
// patrol 71: 44 of 80 segments still had neither end joined to anything.
//
// So the unit here is the **segment** — what the map actually draws — and the question is not "how far apart are
// these points" but "was anything at all being recorded at this instant". Overlapping stretches leave no gap;
// a silence that nobody covered does.
//
// # Where this agrees with the figure, and where it no longer asks
//
// The number above the map comes from `Compute`, which works leg by leg between scans and asks `alongTrack`
// whether a leg was recorded. This function no longer asks that question at all — it cannot, because a silence
// is not a property of a leg. What is preserved is the property that actually matters to a reader: **nothing is
// ever dotted across ground that is drawn solid**. A stretch is either in the chain, and then the chain goes
// around it, or it is discarded as unreconcilable (below), and then it is not on the map's account of the night
// either.
//
// Legs the estimate excludes (too fast, or too long *and* too far — MaxWalkingKmh, MaxLegHours) are still drawn,
// deliberately: the map's claim is about what we recorded, not about what we counted. A scan is a fact even when
// the journey to it was not a walk.
//
// # A recording that cannot be reconciled is dropped, not drawn to
//
// Time is the only thing tying a recording to a scan, and that is not always enough: a member who has left the
// patrol, or a phone with a wrong clock, produces a stretch inside the night that is nowhere near the patrol.
// Drawing a dotted line out to it would claim travel across ground nobody crossed.
//
// So a **step into a stretch** that implies a speed no patrol walks discards the stretch from the chain, and the
// chain continues from the last position we trust — which is how a displaced recording ends up drawn as the
// plain scan-to-scan line it would have had with no track at all. A **step into a scan** is never discarded:
// scans are the ground truth, and a leg between two of them is drawn however implausible, per above.
//
// Order is time order, and there must be two positioned scans for any of this — an unplottable scan anchors
// nothing, as on the page's list. The scans also **bound** the chain: a stretch recorded entirely before the
// first scan or after the last is left out, because a phone left running at home would otherwise be joined to
// the route and drawn as travel hours after the patrol finished.
func UncoveredLegs(scans []Scan, segments [][]Point) []Leg {
	positioned, _ := splitByPosition(scans)
	if len(positioned) < 2 {
		return nil
	}
	sortByTime(positioned)

	known := make([]anchor, 0, len(positioned)+len(segments))
	for _, s := range positioned {
		known = append(known, anchor{first: s, last: s, scan: true})
	}
	// The night, as the scans bound it. A recording outside it is not part of this story — a phone left on at
	// home, or switched on the day before — and joining it would draw a dotted line away from the route and
	// back, hours after the patrol finished.
	from, until := positioned[0].At, positioned[len(positioned)-1].At
	for _, seg := range segments {
		if len(seg) < 2 {
			// Not a stretch, and not on the map either: `patroltrack.Merge` drops a one-point run because
			// a single point is not a line. Nothing to join to.
			continue
		}
		ordered := make([]Point, len(seg))
		copy(ordered, seg)
		sortPointsByTime(ordered)
		first, last := ordered[0], ordered[len(ordered)-1]
		if last.At.Before(from) || first.At.After(until) {
			continue
		}
		known = append(known, anchor{first: first, last: last})
	}
	sortAnchors(known)

	var out []Leg
	// The position and instant at which what we know currently runs out.
	cursor, cursorIsRecording := known[0].last, !known[0].scan

	for _, next := range known[1:] {
		if !next.first.At.After(cursor.At) {
			// No silence: something was already being recorded through this instant. This is the case the
			// point-based version got wrong — two phones overlapping, with no gap between them in time.
			if next.last.At.After(cursor.At) {
				cursor, cursorIsRecording = next.last, !next.scan
			}
			continue
		}

		step := Leg{From: cursor, To: next.first}

		// The reconciliation check, and only where a recording is involved. A scan-to-scan step is drawn
		// however fast it implies, because both of its ends are facts.
		if (!next.scan || cursorIsRecording) && !walkable(step) {
			if next.scan {
				// Trust the scan over the recording we were standing on: draw nothing, and carry on from
				// the scan.
				cursor, cursorIsRecording = next.last, false
				continue
			}
			// A displaced stretch. Leave the cursor where it is, so the chain closes over it.
			continue
		}

		out = appendIfDrawable(out, step)
		cursor, cursorIsRecording = next.last, !next.scan
	}
	return out
}

// anchor is one thing the map knows about a patrol: a scan, or one recorded stretch.
//
// `first` and `last` are where the knowledge begins and ends, and are the same point for a scan — which is what
// lets both kinds sit on one timeline and be joined by the same rule.
type anchor struct {
	first, last Point
	scan        bool
}

// sortAnchors orders by the instant each anchor begins.
//
// Insertion sort, stable, for the same reasons as sortByTime: the input is a few dozen items and nearly in
// order already, and two things recorded in the same second should keep the order they arrived in.
func sortAnchors(items []anchor) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].first.At.Before(items[j-1].first.At); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// minDottedKm is the shortest dotted leg worth drawing: 25 metres.
//
// About the drawing, not about the data. A phone already recording when the patrol reached a checkpoint puts
// its first sample within a few metres of the scan, and a dotted leg that short renders as a smudge under the
// pin — at the zooms this map opens at, 25 m is roughly a pixel. Dropping it loses nothing a reader could
// have seen; keeping it would put marks on the map where there is nothing to say.
const minDottedKm = 0.025

func appendIfDrawable(legs []Leg, leg Leg) []Leg {
	if Between(leg.From.Lat, leg.From.Lng, leg.To.Lat, leg.To.Lng) < minDottedKm {
		return legs
	}
	return append(legs, leg)
}

// walkable reports whether a join could have been made on foot.
//
// The same ceiling the estimate uses for a vehicle leg (MaxWalkingKmh), applied here for a different purpose:
// not to exclude travel we will not count, but to notice that a recording is not where the chain assumes. A
// zero or negative duration is unwalkable unless the two are in the same place, which appendIfDrawable then
// discards anyway.
func walkable(leg Leg) bool {
	km := Between(leg.From.Lat, leg.From.Lng, leg.To.Lat, leg.To.Lng)
	if km < minDottedKm {
		return true
	}
	hours := leg.To.At.Sub(leg.From.At).Hours()
	if hours <= 0 {
		return false
	}
	return km/hours <= MaxWalkingKmh
}
