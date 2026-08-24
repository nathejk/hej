# 049 — Map: tile retry with backoff, and open on the user's location

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Two changes requested after PRD 002 was approved, from using the map:

1. **Grey tiles.** Some tiles fail and stay grey. Leaflet has **no built-in
   retry** — a single failed image request leaves that tile blank until something
   forces the tile to be recreated (pan away and back). Retry failed tiles.
2. **Opening view.** Do not open on the event area: it is not fully known to
   participants and the map should not reveal it. Open on the user's own location,
   falling back to **Sjælland** when no position is available.

PRD 002 updated accordingly (§6 and the "default view" open question is now
resolved).

## Acceptance Criteria

- [x] Failed tiles are retried with exponential backoff + jitter, up to a limit.
- [x] A retried tile that succeeds renders normally (fades in, counts as loaded).
- [x] The error notice appears only after a tile has exhausted its retries — not on
      the first transient failure.
- [x] The notice clears itself once tiles load again.
- [x] Retries are cancelled on layer swap and unmount.
- [x] The map opens centred on the user's position when available.
- [x] Otherwise it frames Sjælland.
- [x] `type-check` clean.

## Progress Log

- 2026-08-24 13:30 — Checked the assumption before coding: fired two bursts of 12
  parallel WMS requests and got **24/24 `200 image/png`**, so the service is not
  rate-limiting us and the failures are intermittent (slow or dropped responses).
  That ruled out throttling as the fix and pointed at retry.
- 2026-08-24 13:35 — Implemented retry in `EventMap.vue`: on `tileerror`, re-assign
  `src` on the **same `<img>`** after `400ms · 2^n` + jitter, up to 3 attempts.
  Reusing the element matters — Leaflet's own load/error handlers stay attached, so
  a late success still marks the tile loaded and fades it in. A `&_retry=n`
  cache-buster defeats negative caching of the failed response. Jitter stops a
  whole screen of failures retrying in lockstep.
- 2026-08-24 13:38 — Retry timers are tracked and cancelled on layer swap and
  unmount, and each retry checks `tile.isConnected`, so a discarded tile is never
  resurrected.
- 2026-08-24 13:40 — Made the error notice honest: it now fires only when a tile
  gives up, and a `tilesOk` event (Leaflet's layer `load`) clears it. Previously one
  transient failure left a warning on screen for the rest of the session.
- 2026-08-24 13:45 — Opening view: `fitBounds` on Sjælland bounds rather than a
  centre+zoom, so it frames sensibly on any screen shape; the user's position wins
  when present. Removed the central-Jutland default.
- 2026-08-24 13:55 — **Verified deterministically** rather than by hoping: a
  headless run aborts the first 6 tile requests outright, then lets traffic through.
  Result: 6 retry requests issued, **8/8 tiles loaded**, no error notice. Without
  the retry those 6 tiles would have stayed grey.
- 2026-08-24 14:00 — Fallback verified by parsing the EPSG:3857 bboxes of the
  requested tiles back to longitudes: centred at 11.25°E and spanning Kalundborg
  (11.09) to Copenhagen (12.57) — Sjælland is genuinely framed. Screenshot confirms
  the whole island. My first assertion here was wrong, not the code: it measured
  requested bboxes, which include Leaflet's off-screen buffer ring.
- 2026-08-24 14:02 — That test also caught something worth keeping: I had set
  `keepBuffer: 3` for smoother panning, which triples off-screen tile fetches — the
  opposite of what we want on rural mobile data. Reverted to Leaflet's default.
- 2026-08-24 14:05 — Observation for the backlog, not fixed here: at national zoom
  the DTK25 raster looks speckled, because it is a 508 DPI 1:25.000 map downsampled
  far beyond its design scale. `topo_skaermkort_DAF` (layer `topo_skaermkort`,
  verified working with the same token) is Dataforsyningen's on-screen map and would
  look right at overview zooms. Adding it as a fourth layer, or auto-swapping below
  ~zoom 11, is a product decision beyond this PRD's three-layer scope.
