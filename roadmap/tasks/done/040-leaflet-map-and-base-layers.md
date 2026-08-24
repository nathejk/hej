# 040 — Leaflet map component and verified Dataforsyningen base layers

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Add the map itself (PRD 002): Leaflet, a lazily loaded `EventMap.vue`, the three
Danish base layers, and the token plumbing.

## Acceptance Criteria

- [x] `leaflet` + `@types/leaflet` installed via the `ui` container.
- [x] `src/config/map.ts` holds the base layers, default view and zoom limits.
- [x] `EventMap.vue` creates the map in `onMounted`, destroys it in
      `onBeforeUnmount`, and is loaded with a dynamic import so Leaflet stays out
      of the app-shell bundle.
- [x] All three base layers render tiles: DTK 1:25.000, DTK 1:50.000, Luftfoto.
- [x] Token read from `VITE_DATAFORSYNINGEN_TOKEN`, not committed.
- [x] `type-check` and `build` clean.

## Progress Log

- 2026-08-24 11:10 — **Resolved PRD 002's biggest unknown by querying the live
  services rather than guessing.** Confirmed with GetCapabilities + GetMap:
  - 1:25.000 → `dtk_25_DAF`, layer `dtk25`
  - 1:50.000 → `dtk_50_DAF`, layer **`dtk_50`**
  - Luftfoto → `orto_foraar_DAF`, layer `orto_foraar`
  Naming is **not** symmetric: `dtk_25_DAF` answers to `DTK25`/`dtk25`/`dtk_25`,
  but `dtk_50_DAF` rejects `DTK50` with a `ServiceException`. Guessing "DTK50" by
  analogy — the obvious move — would have shipped a silently broken layer.
- 2026-08-24 11:12 — Also switched the aerial layer from the sibling repo's Esri
  World Imagery to the **Danish orthophoto**: 12.5 cm Danish imagery instead of a
  mixed global basemap, same provider and token, and it keeps every layer on one
  host (one attribution, one failure mode). Recorded in the PRD.
- 2026-08-24 11:15 — Caveat found in the service metadata and passed upstream: the
  1:50.000 raster is *"opdateres ikke efter år 2017"*. Surfaced in the UI as a
  "Kortdata fra 2017" note under the layer name so a patrol navigating at night
  knows it may not show recent forest/road changes. DTK25 stays the default.
- 2026-08-24 11:20 — Token: build-time `VITE_DATAFORSYNINGEN_TOKEN`, wired through
  `docker-compose.yml` (empty) + the gitignored `docker-compose.override.yml`. No
  BFF proxy and no `/api/map/config`: it is a public quota key for a public
  service, so a proxy would add latency and route tile traffic through the BFF for
  no security gain, and a runtime endpoint ends up in the browser anyway. What
  matters is that it is not committed.
- 2026-08-24 11:25 — Leaflet is kept deliberately outside Vue's reactivity: props
  are watched and translated into imperative Leaflet calls. Base-layer swaps add
  the new layer before removing the old so centre, zoom and overlays survive.
- 2026-08-24 12:50 — **Bug found by looking at a screenshot, not by reading code.**
  The floating controls and the registrations drawer were invisible — buried under
  the tiles — even though the test could still `click()` them, because Leaflet
  gives its panes `z-index: 200–800` and the map container was not a stacking
  context, so those z-indexes competed with app UI in the body context. Fixed with
  `isolate` on the map root (plus explicit `z-10` on the overlays). Added a
  hit-test assertion (`elementFromPoint` at the control's own centre) so
  clickability can never again be mistaken for visibility.
- 2026-08-24 13:05 — ✅ Verified: 9/9 tile responses `200 image/png`, 8 tiles
  rendered, all three layers fetch from their own service, `type-check` and
  `build` clean. Leaflet lands in its own 152.8 kB (44.9 kB gzip) chunk plus 15.7 kB
  of CSS, loaded only on this page — the app shell is unchanged.
