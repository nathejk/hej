# 263 — Edge arrows towards the next checkpoints

**Status:** done
**Priority:** medium
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

PRD 016 phase 3. The feature that makes the map better than paper: while zoomed in on the
patrol's own position, show where the next posts are.

- An arrow appears at the **viewport edge** for a next checkpoint that is outside the
  viewport, in its direction, labelled with distance.
- **"Next"** = the earliest not-yet-scanned *revealed* checkpoints in route order
  `(checkgroup.sortOrder, checkpoint.sortOrder)` — one sequence for every patrol
  (PRD 016 §11.9). A patrol therefore never gets an arrow towards something it has not been
  shown.
- **At most 3** at once, to keep the viewport readable.
- Straight-line bearing and great-circle distance. Not routing (PRD 016 §4).
- Updates on Leaflet `move`/`zoom` and on position change; an arrow disappears when its
  checkpoint enters the viewport.
- Tapping an arrow pans the map to that checkpoint.
- **No own position ⇒ no arrows.** A bearing needs an origin, and the permission card
  already on screen is the explanation.
- **No new geolocation subscription** — consume the existing watch, which is already stopped
  while the page is hidden (PRD 002). Battery on a race night is not negotiable.
- Accessible label per arrow ("Post 5, 3,4 km mod nordøst"); the drawer remains the
  non-visual route to the same information.
- Must not stutter panning on the baseline device (iOS Safari 16.4). Tens of checkpoints,
  not thousands.

## Acceptance Criteria

- [x] `vue/src/components/map/EdgeArrows.vue`.
- [x] Bearing/distance maths unit-tested against known coordinate pairs.
- [x] Off-screen → arrow; on-screen → none (test).
- [x] Capped at 3, choosing the earliest in route order (test).
- [x] No position ⇒ no arrows (test).
- [x] Tap pans to the checkpoint.
- [x] Accessible labels present, in Danish, with `da-DK` distances.
- [ ] Pan/zoom on device does not stutter — **needs a device**. The design keeps it cheap (a handful
      of trig calls over ≤ 3 targets per move, no new geolocation subscription, no formatter
      constructed per frame), but "does not stutter on an iPad" is not something a test run can
      claim. Carried to task 264's device pass.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 3.
- 2026-09-15 — Built in four pieces, three of them pure and tested in node: `arrowGeometry.ts` (haversine,
  bearing, edge intersection, Danish formatting), `nextCheckpoints.ts` (which posts), `arrowPlacement.ts`
  (whether and where), and `EdgeArrows.vue` (markup only).

  The split is not tidiness. Vitest here runs without a DOM, so the alternative to extracting the rules is
  **not testing them** — and every one of them is a decision a patrol is affected by at 02:00 in a forest.
- 2026-09-15 — Decision: "next" is **route order, not distance**. The nearest revealed post is often not the
  next one — a patrol walking a loop passes close to posts it has already visited and posts several legs ahead
  — so pointing at the nearest would send them backwards.
- 2026-09-15 — Decision: a visited post is skipped even when earlier posts are unvisited. Patrols legitimately
  reach posts out of order, and continuing to point at somewhere they have already stood is worse than silence.
- 2026-09-15 — Decision: `viewportChanged` is emitted as a **bare signal** and the overlay calls back through
  `project()`, rather than the map emitting coordinates. Projection depends on the live centre, zoom and
  container size; an overlay that reimplemented it would drift on every animated pan and would have to know
  about Web Mercator — which is exactly the knowledge `EventMap` exists to contain.
- 2026-09-15 — The revision counter is the seam between Leaflet's imperative world and Vue's reactive one.
  Leaflet's state is deliberately not reactive, so there is nothing to watch; the counter is the smallest thing
  that can stand in for "it moved", and it is what makes the arrows track a pan continuously instead of
  snapping when the finger lifts.
- 2026-09-15 — **A test failed and the code was right — geodesy, not a bug.** I asserted that a due-east
  bearing is exactly 90°; it is 89.59° at 56°N. A parallel is not a great circle, so the shortest path due
  "east" sets off slightly north and curves. Fixed the tolerance and wrote the reason into the test, since the
  next person to see 89.59 will have the same doubt.
- 2026-09-15 — **Two more failures, both my fixture's fault.** The fake projection hardcoded one viewport size,
  so a test using a wider viewport placed the patrol's own position outside the box — which the code correctly
  refuses to draw arrows from. And it coupled pixel and ground scale, so "440 m away" was also "on screen",
  making a distance-formatting test unsatisfiable. The projection now takes the size and a `pxPerDegree`
  standing in for zoom, with a comment explaining that a distance test has to pick a plausible zoom.
- 2026-09-15 — One criterion is honestly **unchecked**: "pan/zoom does not stutter" needs a device. The design
  keeps the per-move cost to a few trig calls over at most three targets, reuses the existing geolocation watch
  (no new subscription, so no battery cost), and avoids constructing an `Intl` formatter per frame — but a test
  run cannot claim smoothness on an iPad. Carried to task 264.
- 2026-09-15 — ✅ 43 new specs across the three pure modules; `type-check` clean, 589 frontend tests passing,
  build clean with Leaflet still in its own lazy chunk.
- 2026-09-15 — Done.
