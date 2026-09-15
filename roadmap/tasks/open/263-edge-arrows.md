# 263 — Edge arrows towards the next checkpoints

**Status:** open
**Priority:** medium
**Created:** 2026-09-15

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

- [ ] `vue/src/components/map/EdgeArrows.vue`.
- [ ] Bearing/distance maths unit-tested against known coordinate pairs.
- [ ] Off-screen → arrow; on-screen → none (test).
- [ ] Capped at 3, choosing the earliest in route order (test).
- [ ] No position ⇒ no arrows (test).
- [ ] Tap pans to the checkpoint.
- [ ] Accessible labels present, in Danish, with `da-DK` distances.
- [ ] Pan/zoom on device does not stutter.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 3.
