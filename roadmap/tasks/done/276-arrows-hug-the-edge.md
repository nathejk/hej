# 276 — Arrows float in mid-air instead of hugging the viewport edge

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

Bug reported by the product owner, with screenshots: the arrows point at the right posts now, but they sit
well *inside* the viewport rather than on its edge — "quite far from the edge, but while pan and zoom I can
make them go closer".

The cause was my own task 264. To keep arrows clear of the floating controls I reserved a **full-width
band** — 112 px off the top, 96 off the bottom — and clamped every arrow into the middle strip that was
left. But the controls are in **corners**, not across whole edges: the layer/locate stack is top-right, the
registrations handle is bottom-centre, and the notices are top-left and only sometimes present. So an arrow
leaving the top edge on the *left*, where nothing floats, was shoved 112 px inward and hung in mid-air.

The "pan and zoom brings it closer" clue fits exactly: panning until an arrow exits a *side* edge — inset
only 8 px — put it back near the edge, while any arrow exiting top or bottom floated.

**Fix:** hug the edge everywhere, and clear the controls by sliding the arrow *along* its edge past them:

- Placement inset is just `ARROW_RADIUS + 8`, so every arrow hugs the edge.
- The keep-out is now a set of **rectangles** (`arrowKeepOutZones`) anchored to the corners the controls
  occupy — top-right stack, bottom-centre handle — safe-area aware. An arrow landing on one slides along its
  edge to the nearer clear side (`slideClear`); the rest of every edge is free.
- The **notices are not a zone**. They are top-left, conditional, and reserving space for them is precisely
  what made the top-left float. A rare transient overlap with a notice is the better trade.

## Acceptance Criteria

- [x] An arrow on an edge with no control hugs the edge (test).
- [x] An arrow that would sit under the top-right controls slides along the edge to clear them (test).
- [x] After sliding, it still hugs the edge — the slide is tangential, not inward (test).
- [x] An arrow already clear of every control is not moved at all (test).
- [x] Sliding changes neither bearing nor distance (test).
- [x] Every arrow stays on screen with the zones applied (test).
- [x] The notices corner (top-left) is left free (test).
- [x] Frontend suite, type-check, build and all four Go gates clean.

## Progress Log

- 2026-09-15 — Reported with two screenshots. Worked back from the "pan brings it closer" clue: a constant
  keep-out cannot explain a distance-from-edge that changes with panning, so the arrow was being pushed by a
  band whose size differed per edge — which is exactly what task 264's `arrowKeepOut` did (112 top, 8 sides).
- 2026-09-15 — Replaced the band model with corner **zones** and edge **sliding**. This is the standard
  off-screen-indicator behaviour and the thing task 264 should have been: an arrow hugs the edge and only
  steps aside where a control actually is.
- 2026-09-15 — Decision: the top-left notices get **no zone**. Reserving for a conditional, corner-local
  element is what floated the left of the top edge; the notices are semi-transparent and an arrow briefly
  under a notice that is only sometimes there is a far smaller cost than the bug being fixed.
- 2026-09-15 — Decision: the top-right zone starts at `y = 0`, not at the safe-area inset, so it also covers
  the notch area above the controls' own inset — an arrow grazing the very top-right corner is caught rather
  than tucked behind the status bar next to the buttons.
- 2026-09-15 — This is the **third correction to the arrows** in one sitting (274 vanishing/jumping, 275
  wrong targets, 276 placement). All three shared a property: invisible to the test suite as written and to a
  static screenshot, visible only with a finger on the glass. Recorded plainly — it is the strongest
  argument yet that the deferred device pass (tasks 263/264) is not optional, and I have stopped treating
  "tests green" as "done" for this component.
- 2026-09-15 — ✅ 26 placement specs + 10 zone specs; 609 frontend tests, type-check and build clean; all four
  Go gates clean.
- 2026-09-15 — Done.
