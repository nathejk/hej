# 039 — Full-bleed route support in the app shell

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

PRD 002 requires the map to use everything above the bottom nav. Add a declarative
way for a route to opt into edge-to-edge rendering instead of special-casing route
names inside `App.vue`.

## Acceptance Criteria

- [x] `NavDestination` gains `fullBleed?: boolean`; `/maps` sets it.
- [x] The router copies it into route `meta`, typed on `RouteMeta`.
- [x] `App.vue` hides the top bar and drops the `overflow-y-auto` wrapper for
      full-bleed routes, using `relative` so the view can position overlays.
- [x] Non-full-bleed routes are unchanged (header + scrolling `main`).
- [x] Sign-out stays reachable on every other page.

## Progress Log

- 2026-08-24 12:05 — Implemented via `config/navigation.ts` → route `meta` →
  `App.vue`, keeping navigation declarative (one source for label/icon/roles/
  layout) rather than testing `route.name === 'maps'` in the shell.
- 2026-08-24 12:40 — ✅ Verified in a headless browser at 390×844: no `<header>`
  present on `/maps`, the Leaflet container starts at y=0 with height 787, and the
  bottom nav begins at exactly 787 — i.e. the map fills the shell with no gap and
  no overlap.
- 2026-08-24 12:41 — Sign-out is now unreachable *from the map page specifically*.
  That is intentional per PRD 002/003 (it moves to the profile page), but until
  PRD 003 ships, users landing on `/maps` must switch tabs to sign out. Noted for
  the PRD-003 discussion rather than worked around here.
