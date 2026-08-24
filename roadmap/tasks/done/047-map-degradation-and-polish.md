# 047 — Map degradation, polish and tile-caching guard

**Status:** done
**Priority:** medium
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

The failure paths PRD 002 insists on: the map must stay usable when tiles fail,
when the position cannot be found, or when the token is missing. Plus the
service-worker guard against caching third-party tiles.

## Acceptance Criteria

- [x] Tile failure shows a dismissible-style notice and leaves position + markers
      working.
- [x] Missing token is reported explicitly rather than showing blank tiles.
- [x] Position timeout/denial shows a message; no blocking dialogs anywhere.
- [x] Controls ≥44px with `aria-label`s; notices never cover the controls.
- [x] Map tiles are not precached or runtime-cached by the service worker.

## Progress Log

- 2026-08-24 12:30 — Three separate notices, because they need different answers:
  missing token (a config error, names the env var), tile fetch failure ("Din
  placering virker stadig" — tells the user what still works), and position failure.
  All non-blocking, top-left so they never overlap the top-right controls.
- 2026-08-24 12:32 — Service worker: verified `vite.config.ts` configures **no**
  `runtimeCaching`, and Workbox `generateSW` only precaches build assets — so WMS
  tiles are never cached. Confirmed against the build output: 24 precache entries,
  all local assets, no `api.dataforsyningen.dk`. The risk would be adding
  `runtimeCaching` later; recorded in the `vue3-pwa-layout` skill's don'ts.
- 2026-08-24 12:55 — The tile-failure path got verified for real by accident: one
  headless run hit a transient WMS failure and the map correctly showed
  "Kortbilleder kunne ikke hentes. Din placering virker stadig." with the position
  dot, controls and registrations pill all still working. Better evidence than a
  forced test.
- 2026-08-24 13:05 — ✅ Final run clean: 9/9 tiles `200 image/png`, both controls
  hit-test topmost at exactly 44×44, zero console errors, zero page errors.
