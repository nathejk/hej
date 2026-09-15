# 274 — Arrows vanish and jump during pan and zoom

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

Bug reported by the product owner against task 263:

> when pan and zoom, the arrows disappear from time to time, placement of the arrows are jumping
> around, they are supposed to be just inside the viewport

Both symptoms had one cause. `computeArrows` placed each arrow where the line **from the patrol's own
position** to the post crossed the screen edge. That reads well while the map is following them — they sit
in the middle, so the line leaves the screen exactly where the post is — and it fails as soon as anyone
pans or zooms:

- **Vanishing.** A ray needs an origin *inside* the rectangle to have an edge crossing, and
  `edgeIntersection` correctly returns `null` otherwise. A pan, or a zoom-out, routinely puts the patrol's
  own position off screen — so every arrow disappeared at once, precisely when the user was looking around
  for their next post.
- **Jumping.** During a drag the patrol's dot slides across the screen, so the ray pivoted around it and
  the crossing point raced along the edge far faster than the map moved underneath.

**Fix:** measure *placement* from the **centre of the viewport**. In container coordinates the centre never
moves and is always inside the box, so an off-screen post is always placeable and its arrow tracks it
smoothly as the map slides. Where the map is following the patrol, centre and patrol coincide — which is
the case the original geometry was tuned for, so nothing is lost there.

**Bearing and distance still come from the patrol.** Those are facts about the ground: the chevron means
"walk this way" and the label says how far, and neither may change because somebody dragged the map.

## Acceptance Criteria

- [x] An arrow survives the patrol's own position leaving the screen.
- [x] An arrow exists whenever its post is off screen, across a full pan sweep in both axes and both
      directions.
- [x] Arrows survive a zoom-out that sweeps the patrol out of the viewport.
- [x] A small pan moves an arrow by a comparable amount, not the length of an edge.
- [x] Every arrow sits just inside the viewport, asserted from all directions rather than one quadrant.
- [x] Bearing, distance and label are unchanged by panning.
- [x] The new tests fail against the old geometry (verified by reverting it).
- [x] Frontend suite, type-check and build clean.

## Progress Log

- 2026-09-15 — Reported. Read `computeArrows` before theorising: the origin passed to `edgeIntersection`
  was `project(patrol)`, and the function bails when the origin is outside the rectangle — which is both
  symptoms in one line.
- 2026-09-15 — Fixed by using the viewport centre as the placement origin, with the reasoning written into
  the function's doc comment rather than only here: the next person to read it will otherwise wonder why
  placement and bearing are measured from different points, which is the one surprising thing about it.
- 2026-09-15 — Decision: **bearing and distance stay patrol-relative.** Placement is a screen question and
  belongs to the centre; direction and distance are ground questions and belong to the patrol. Conflating
  them would have made the chevron and the spoken label drift apart as the map moved, which is a subtler
  bug than the one being fixed. There is a test that panning changes neither.
- 2026-09-15 — Inverted the test that had encoded the bug (`draws nothing when the patrol's own position is
  off screen`) rather than deleting it, so the reversal is visible in the diff.
- 2026-09-15 — **My first new test was wrong and failed honestly.** It asserted an arrow at every step of a
  pan; panning *towards* a post eventually brings it on screen, where suppressing the arrow is correct and
  deliberate. Rewrote it around the real invariant — "an arrow exists whenever the post is off screen" —
  computed from the post's own projected position, which is a stronger assertion than the one I first
  reached for.
- 2026-09-15 — **Verified the tests can fail.** Temporarily restored the patrol-relative origin: 5 of the
  23 placement tests failed, covering both reported symptoms. Restored; all 23 pass.
- 2026-09-15 — ✅ 610 frontend tests, type-check and build clean.
- 2026-09-15 — Note for the outstanding device pass (tasks 263/264): this bug was invisible to the test
  suite as written *and* to a static screenshot — it only appears while a finger is on the glass. Worth
  remembering when judging how much the remaining "does not stutter" criterion is really worth.
- 2026-09-15 — Done.
