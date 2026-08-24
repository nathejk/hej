# 042 — Own position: continuous watch, marker and locate button

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Show the user on the map and keep it current, without flattening the battery
(PRD 002). Extends the `location.store` plumbing that PRD 001 deliberately left
unconsumed.

## Acceptance Criteria

- [x] `location.store` gains `watch()` / `stopWatch()` (single subscription) and a
      `following` flag.
- [x] Position rendered as a distinct marker plus an accuracy circle, updated live.
- [x] Locate button recentres and re-enables follow; manual panning disables it.
- [x] The watch is suspended while the page is hidden and resumed on return.
- [x] The existing soft permission pre-prompt behaviour is preserved.
- [x] Denied/unavailable permission leaves a fully usable map.

## Progress Log

- 2026-08-24 11:45 — Added `watch()`/`stopWatch()`/`setFollowing()` to
  `location.store`, reusing the existing permission handling. A denied watch drops
  its own subscription rather than holding a dead one, so a later grant can start
  fresh.
- 2026-08-24 11:50 — Battery: the high-accuracy watch runs only while the document
  is visible (`visibilitychange`), and is torn down on unmount.
- 2026-08-24 11:55 — Follow mode needed care: Leaflet fires `dragstart`/`zoomstart`
  for programmatic moves too, so our own `setView` calls would have instantly
  cancelled follow. Guarded with a `selfMoving` flag around every self-initiated
  move.
- 2026-08-24 12:00 — `LocateButton.vue` reflects three states — following, idle and
  **blocked** — so a user who denied permission is told why the map will not find
  them instead of tapping a dead button.
- 2026-08-24 13:05 — ✅ Verified with a simulated position (56.19, 9.47): the
  position dot and accuracy ring render, the button hit-tests topmost at 44×44 and
  shows the "following" state, and the map opened centred on the position.
- 2026-08-24 13:06 — Not verified here: real GPS drift/accuracy behaviour outdoors,
  and battery cost over an evening. Those need a field test, not emulation.
