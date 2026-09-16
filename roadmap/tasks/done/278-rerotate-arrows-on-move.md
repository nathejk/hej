# 278 — Re-rotate the edge arrows toward the checkpoint on every map move

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

Reported by the product owner: on every pan/move the arrows should re-rotate so they keep pointing at the
checkpoint.

They did not. The chevron's rotation was the **geographic bearing from the patrol to the post** — a fact
about the ground that, by design (task 274), does not change when the map moves. The *placement* followed
the map, but the rotation stayed fixed, so once the map was panned the arrow sat at the edge pointing in a
direction that no longer aimed at where the post actually was on screen.

This reverses the task-274 decision that "bearing is a ground fact, independent of viewport", and the
reversal is right: an edge arrow is an **off-screen indicator**, and its one job is to point at where the
thing is. That is a screen-space direction and it must track the map.

**Fix:** the rotation is now the screen-space angle from the arrow's placed position to the checkpoint's
*projected* position (`screenBearingDegrees`). It is recomputed inside `computeArrows`, which the overlay
already re-runs on every Leaflet `move` (via the `revision` counter from task 274) — so it re-aims on every
pan and zoom, and again after an arrow slides along the edge to clear a control.

What did **not** change: the **distance** is still patrol-to-post, a ground fact, so it stays put when the
map is dragged. On the north-up map this app uses, screen-up is north, so the screen bearing doubles as a
compass bearing and the accessible label ("mod nordøst") is derived from the same number as the chevron —
they cannot disagree.

## Acceptance Criteria

- [x] The chevron points at the checkpoint's on-screen location (test, with a fixed projection).
- [x] The rotation re-aims when the map is panned (test).
- [x] The rotation re-aims when an arrow slides along an edge to clear a control (test).
- [x] The distance does *not* change on a pan (test).
- [x] `screenBearingDegrees` covered directly: cardinals, no negative angle, coincident points, agrees with
      the compass on a north-up map.
- [x] Frontend suite, type-check and build clean.

## Progress Log

- 2026-09-15 — Diagnosed: `computeArrows` set `bearing = bearingDegrees(patrol, cp)`, a ground bearing that
  is deliberately viewport-independent. Placement tracked the map; rotation did not. So a panned arrow
  pointed "the way to walk from the patrol" rather than "at the post on screen", which is what an edge arrow
  should do.
- 2026-09-15 — Added `screenBearingDegrees(from, to)` to arrowGeometry: the angle clockwise from up between
  two screen points, `atan2(dx, -dy)` because the chevron graphic points up at 0°. Rotation is now measured
  from the arrow's *placed* point to the projected target, so an arrow that slid to clear a control still
  aims at the post from where it ended up.
- 2026-09-15 — Consciously reversing task 274's "bearing is a ground fact" call. It was defensible for a
  "walk this way" reading, but the product decision is that the arrow points at the checkpoint, and an
  off-screen indicator that does not track the map is the bug. Recorded in PRD 016 §11.15 so the reversal is
  legible rather than looking like drift.
- 2026-09-15 — Removed `bearingDegrees` and its tests: nothing calls it any more. Left in place it would be a
  well-tested, exported function that no production code uses — an invitation to wire the old behaviour back.
  (staticcheck would not have flagged it, since it is exported; this is a judgement call, not a gate.)
- 2026-09-15 — Strengthened two slide tests while here. They used a target (`ne` 57,12) whose edge crossing
  landed *below* the control stack, so they passed without the slide ever running. Swapped to a target that
  provably lands within the stack's band (right edge at y ≈ 94, inside 0–167), so the slide is actually
  exercised. The old tests were green for the wrong reason — the same class of weak test I have hit twice in
  this feature.
- 2026-09-15 — ✅ 27 geometry specs, 26 placement specs; 611 frontend tests, type-check and build clean.
- 2026-09-15 — Done.
