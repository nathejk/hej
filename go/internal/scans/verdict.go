package scans

// The on-time verdict for a checkpoint scan (PRD 016 §11.5/§11.6).
//
// # Why this lives here and is computed, not stored
//
// "Were we on time?" is derived, and it must be derived on the **server**: a device with a wrong clock
// could otherwise invent lateness, and the whole point of the badge is that a patrol trusts it. It lives in
// this package because `relative` windows anchor on the patrol's *own* scan at another checkgroup, and the
// patrol's whole set of scans is exactly what this package assembles.
//
// # The rule matches the organizers' own screens, exactly
//
// Inclusive bounds, and **no grace period**. HQ's live path (`scansByCheckgroup`) is a plain
// `uts BETWEEN openFrom AND openUntil`; the `minusMinutes`/`plusMinutes` grace belongs to a superseded
// control-group model whose only copy is commented out, and the current `checkpoint` table has no grace
// columns. Applying a margin here would put the app and HQ into quiet disagreement about who was on time,
// which is worse than either rule alone.

// SchemeFixed / SchemeRelative / SchemeNone are the checkgroup's window schemes, as the checkgroup
// projection stores them. Duplicated as plain strings rather than imported, because internal/scans must not
// depend on nathejk/table (the projection adapts *into* this package, one way only).
const (
	SchemeFixed    = "fixed"
	SchemeRelative = "relative"
	SchemeNone     = "none"
)

// Verdict is whether a scan landed inside its post's open window.
//
// Present (non-nil on a Scan) only when there is a window to judge against. Absent for `none`, for a
// checkpoint with no window recorded yet, for a `relative` window whose anchoring scan has not happened, and
// for a scan the rota could not attribute to any post — because a missing verdict is honest where a guessed
// one ("for sent" at a post with no hours set) is a lie the patrol would act on.
type Verdict struct {
	// OnTime is true when the scan is within [openFrom, openUntil] inclusive.
	OnTime bool
	// DeltaSeconds is signed: 0 exactly on time, negative when early (before the window opened), positive
	// when late (after it closed). The magnitude is what the drawer shows as "12 min for sent" — a number a
	// patrol can act on, unlike a bare verdict.
	DeltaSeconds int

	// SpentSeconds is how long the leg took: the gap between the patrol's scan at the anchoring checkgroup
	// and this one.
	//
	// **Only set for a `relative` window**, and nil otherwise, because it is only meaningful there. A
	// relative window opens at the patrol's own arrival at the previous line, so the distance from that
	// instant to this one is exactly "how long this leg took them" — a fact about their walk. A `fixed`
	// window is an absolute clock time set by organizers, and the gap from it measures nothing about the
	// patrol: two patrols arriving together would get different numbers depending on when the post opened.
	// Reporting a figure there would invite the drawer to render a duration that means nothing.
	//
	// Reported whether or not they were on time, because the elapsed time is a fact either way and the
	// client decides what to show.
	SpentSeconds *int
}

// windowFor resolves a checkpoint's open window to absolute instants, or reports that there is none.
//
// `anchorUts`/`hasAnchor` carry the patrol's scan at the relative checkgroup, which only the caller can look
// up (it is another of the patrol's scans). A relative window with no anchor, or a fixed window with no
// recorded hours, has no window — ok is false, and the caller produces no verdict.
func windowFor(
	scheme string,
	openFromUts, openUntilUts int64,
	openDurationMin int,
	anchorUts int64,
	hasAnchor bool,
) (from, until int64, ok bool) {
	switch scheme {
	case SchemeFixed:
		// 0 means "no hours recorded" (task 252 stores 0, not the epoch), so it is not a window.
		if openFromUts == 0 || openUntilUts == 0 {
			return 0, 0, false
		}
		return openFromUts, openUntilUts, true
	case SchemeRelative:
		if !hasAnchor || openDurationMin <= 0 {
			return 0, 0, false
		}
		return anchorUts, anchorUts + int64(openDurationMin)*60, true
	default:
		// SchemeNone, "" (not seen yet), or a scheme this build does not recognise: no verdict. Never
		// guess one from a scheme we do not understand.
		return 0, 0, false
	}
}

// verdictFor computes the verdict for one scan, or nil when there is no window to judge against.
func verdictFor(
	scanUts int64,
	scheme string,
	openFromUts, openUntilUts int64,
	openDurationMin int,
	anchorUts int64,
	hasAnchor bool,
) *Verdict {
	from, until, ok := windowFor(scheme, openFromUts, openUntilUts, openDurationMin, anchorUts, hasAnchor)
	if !ok {
		return nil
	}

	// Only a relative window measures from the patrol's own arrival, so only there does "time spent" mean
	// anything. Clamped at zero: a scan before its own anchor is a clock or ordering anomaly, and a
	// negative duration would render as nonsense rather than as the anomaly it is.
	var spent *int
	if scheme == SchemeRelative && hasAnchor {
		s := int(scanUts - anchorUts)
		if s < 0 {
			s = 0
		}
		spent = &s
	}

	switch {
	case scanUts < from:
		return &Verdict{OnTime: false, DeltaSeconds: int(scanUts - from), SpentSeconds: spent} // negative: early
	case scanUts > until:
		return &Verdict{OnTime: false, DeltaSeconds: int(scanUts - until), SpentSeconds: spent} // positive: late
	default:
		// Inclusive on both ends, matching HQ.
		return &Verdict{OnTime: true, DeltaSeconds: 0, SpentSeconds: spent}
	}
}
