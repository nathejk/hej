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
- 2026-08-26 — **Follow-up fix: the aerial layer was requesting PNG.** Found while
  measuring tile sizes for PRD 009's storage budget, by fetching real tiles from the
  live service rather than estimating. Mean bytes per 256 px tile, central Zealand:

  | zoom | DTK25 (PNG) | aerial (PNG) | aerial (**JPEG**) |
  |---|---|---|---|
  | 12 | 140 kB | 154 kB | **14 kB** |
  | 14 | 99 kB | 145 kB | **11 kB** |
  | 16 | 32 kB | 137 kB | **9 kB** |

  `format` was hardcoded to `image/png` for every layer, so the aerial layer cost
  **~15x more bytes than necessary** — losslessly encoding photographic detail. That is
  live mobile-data cost on rural connections, which is this app's whole context, and it
  would have dominated PRD 009's offline budget. `format` is now per-layer on
  `BaseLayerConfig`: JPEG for the orthophoto, PNG kept for the topographic layers, whose
  line art and place names JPEG would smear.

  **A trap found in passing, and commented in the code.** Dataforsyningen's WMS accepts
  Leaflet's lowercase `transparent=false` under WMS **1.1.1** (Leaflet's default) but
  **rejects it under 1.3.0** with `ServiceException: TRANSPARENT must be either TRUE or
  FALSE`. Leaflet sends that parameter on every tile request, so adding
  `version: '1.3.0'` — an obvious-looking modernisation — would break all three layers at
  once, and the failure arrives as a `200` containing XML rather than as an HTTP error.
  Verified against the live service; the warning now sits next to the option.

  Also confirmed while here that the runtime token delivery works end to end: the token
  reaches `/api/config`, fetches a real tile, and is **no longer baked into the bundle**
  (an older local `dist/` still had the build-time value; `vue/dist/` is gitignored and
  the token is not in git history). `type-check` and `build` clean.
