// Map configuration: base layers, default view and zoom limits.
//
// # The layer definitions live in `public/maplayers.json`, not here (task 353)
//
// Because two maps have to draw the same thing: this app's `EventMap.vue`, and the **public map island**
// (PRD 011 §8) — a ~200-line vanilla script on a page that deliberately loads no bundle, so it cannot
// import anything from `src/`. It used to carry a hand-copied WMS URL, and the copy had drifted to a
// *fourth* service nobody in the app had ever seen.
//
// So the values sit in a plain JSON file that this module imports at build time and the island fetches at
// runtime. The reasoning for each value — which service answers to which layer name, and why the aerial
// layer is JPEG while the topographic ones are PNG — is in that file, next to the values it explains.
//
// The service paths and WMS layer names were verified against live GetCapabilities + GetMap responses on
// 2026-08-24. See PRD 002 §11.

import mapLayersFile from '../../public/maplayers.json'

export interface BaseLayerConfig {
  /** Label shown in the layer switcher (Danish). */
  label: string
  /** WMS endpoint. */
  url: string
  /** WMS layer name. */
  layer: string
  attribution: string
  /**
   * WMS output format — `image/png` for the topographic layers, `image/jpeg` for the aerial one.
   *
   * Per layer rather than global, because the difference is ~15× in bytes for aerial imagery. The
   * measurement and the reasoning are in `public/maplayers.json`.
   */
  format: 'image/png' | 'image/jpeg'
  /** Extra note surfaced in the switcher, e.g. data currency caveats. */
  note?: string
}

// The Dataforsyningen token is not inlined here: it is fetched at runtime from
// GET /api/config (see config/runtime.ts) so the same built image can be
// deployed with a different key. When it is missing the map reports it instead
// of silently showing grey tiles.

// Keys are stable identifiers persisted in localStorage — renaming one resets
// the user's layer choice, so don't.
export type BaseLayerKey = 'dtk25' | 'dtk50' | 'orto'

// The shared file's shape, as far as this module cares. A JSON import is typed structurally, so `format`
// arrives as `string` and is narrowed once, here, rather than asserted at every use.
interface MapLayersFile {
  attribution: string
  default: string
  minZoom: number
  maxZoom: number
  retry: { limit: number; baseDelayMs: number; jitterMs: number }
  layers: Record<string, { label: string; url: string; layer: string; format: string; note?: string }>
}

const file = mapLayersFile as unknown as MapLayersFile

// The attribution is stored once in the file and attached to every layer here, because Leaflet takes it per
// layer. `mapLayers.spec.ts` asserts every key in `BaseLayerKey` is present — the check the old
// `satisfies Record<BaseLayerKey, BaseLayerConfig>` gave for free and a JSON import cannot.
export const baseLayers = Object.fromEntries(
  Object.entries(file.layers).map(([key, cfg]) => [
    key,
    {
      label: cfg.label,
      url: cfg.url,
      layer: cfg.layer,
      attribution: file.attribution,
      format: cfg.format as BaseLayerConfig['format'],
      ...(cfg.note ? { note: cfg.note } : {}),
    } satisfies BaseLayerConfig,
  ]),
) as Record<BaseLayerKey, BaseLayerConfig>

export const DEFAULT_BASE_LAYER = file.default as BaseLayerKey

/**
 * The WMS options a tile layer is built with — shared by the map and the offline downloader.
 *
 * **This function exists so the two cannot drift.** Task 087's bulk download has to produce byte-identical
 * URLs to the ones the map requests, or it fills the cache with entries the map will never look up: the
 * service worker matches on the whole URL, and only `token` and `_retry` are normalised away
 * (`TILE_CACHE_KEY_IGNORED_PARAMS`). Any other difference — a parameter present, absent, or in another
 * order — means a few hundred megabytes downloaded and then ignored, which would look exactly like a
 * working feature until somebody opened the map in a forest.
 *
 * Every option below has a reason recorded at the one place it is set:
 *
 *  - `format` is per-layer, because the aerial layer is ~15× smaller as JPEG than as PNG.
 *  - `crossOrigin` makes tiles CORS-readable so the worker can store them as ordinary responses.
 *    Without it they are **opaque**, and browsers pad opaque responses for quota accounting to prevent
 *    cross-origin size leaks — turning a 324 MB tile set into something far larger against the same
 *    quota. Safe because Dataforsyningen reflects `Access-Control-Allow-Origin` (verified against the
 *    live service for the dev and production hostnames, 2026-08-26).
 *  - `transparent: false` and the implied WMS **1.1.1**: do not add `version: '1.3.0'` without
 *    uppercasing the value. Leaflet emits lowercase parameter values, and 1.3.0 makes the same service
 *    answer `ServiceException: TRANSPARENT must be either TRUE or FALSE` — on *every* tile, arriving as
 *    a 200 containing XML rather than as an HTTP error.
 *  - `token` is not a WMS parameter; Leaflet copies unrecognised options into the query string, which is
 *    how the Dataforsyningen key reaches the service.
 */
export function wmsLayerOptions(cfg: BaseLayerConfig, token: string) {
  return {
    layers: cfg.layer,
    format: cfg.format,
    crossOrigin: 'anonymous' as const,
    transparent: false,
    attribution: cfg.attribution,
    token,
    maxZoom: MAX_ZOOM,
  }
}

/** localStorage key for the user's base-layer choice. */
export const BASE_LAYER_STORAGE_KEY = 'hej.map.baseLayer'

// Opening view: see FALLBACK_BOUNDS below — the map centres on the user when a
// position is available. Both zoom limits come from the shared file, so the public map cannot open at a
// zoom this one refuses.
export const MIN_ZOOM = file.minZoom
export const MAX_ZOOM = file.maxZoom
/** Zoom used when recentring on the user's own position. */
export const LOCATE_ZOOM = 15

// Fallback view when we have no position yet: Sjælland. The event area is not
// fully known to participants, so we deliberately do not reveal it — the map
// opens on the user's own location when it can, and on Sjælland when it cannot.
// Bounds rather than a centre+zoom so it frames sensibly on any screen shape.
export const FALLBACK_BOUNDS: [[number, number], [number, number]] = [
  [55.05, 10.95],
  [56.2, 12.75],
]

// Only used for the initial map construction, before FALLBACK_BOUNDS is applied.
export const FALLBACK_CENTER: [number, number] = [55.6, 11.85]
export const FALLBACK_ZOOM = 8

// Tile retry policy, shared with the public map island through `public/maplayers.json`. Leaflet has no
// built-in retry: a single failed image request leaves that tile grey until the user pans away and back.
// On patchy rural mobile data that is the normal case, not the exception, so failed tiles are retried
// with backoff before we admit defeat.
export const TILE_RETRY_LIMIT = file.retry.limit
export const TILE_RETRY_BASE_DELAY_MS = file.retry.baseDelayMs
/** Upper bound on the random jitter added to each backoff, so a screen of tiles does not retry in lockstep. */
export const TILE_RETRY_JITTER_MS = file.retry.jitterMs

// Where the map's floating controls sit, so edge arrows can avoid them (PRD 016, task 264; corrected task
// 276).
//
// `MapsView` floats things over the map in the same layer the arrows use, and an arrow under the locate
// button is worse than no arrow — it is invisible *and* it steals the tap. But the controls are **in the
// corners**, not across whole edges: the layer/locate stack is top-right, the registrations handle is
// bottom-centre. The notices are top-left and only sometimes present.
//
// The first version (task 264) reserved a full-width band — 112 px off the top, 96 off the bottom — and that
// broke the feature it was protecting: an arrow leaving the top edge on the *left*, where nothing floats, was
// shoved 112 px inward and hung in mid-air instead of hugging the edge. So the keep-out is now a set of
// **rectangles** the arrow slides along the edge to clear (see arrowPlacement.slideClear), and the rest of
// every edge is free.
//
// The notices are deliberately **not** a zone. They are top-left, conditional, and reserving space for them
// is exactly what made the left of the top edge float; a rare transient overlap with a notice is the better
// trade. Sizes are anchored to the corners the controls actually occupy, safe-area aware because `--sat` is
// 59 px on a notched phone and 0 on desktop.

/** A rectangle in the map container's pixel space. */
export interface KeepOutZone {
  x: number
  y: number
  width: number
  height: number
}

// The control stack's footprint: a couple of stacked ≥ 44 px targets, inset ~12 px from the top-right corner.
const CONTROL_STACK = { width: 60, height: 108, margin: 12 } as const
// The registrations handle: a pill centred on the bottom edge, above the safe-area inset.
const HANDLE = { width: 240, height: 56, margin: 12 } as const

/**
 * The rectangles an edge arrow must not sit on, for a given viewport and safe-area reading.
 *
 * Pure and viewport-relative, so it can be tested without a browser — the reading and the size need a device,
 * the arithmetic does not.
 */
export function arrowKeepOutZones(
  size: { width: number; height: number },
  safe: { top: number; bottom: number },
): KeepOutZone[] {
  const top = usableInset(safe.top)
  const bottom = usableInset(safe.bottom)

  return [
    // Top-right: the layer switcher and locate button. From the very top edge (y = 0) down past the
    // controls, so an arrow grazing the top-right corner is caught even in the notch area above the
    // controls' own inset.
    {
      x: size.width - CONTROL_STACK.width - CONTROL_STACK.margin,
      y: 0,
      width: CONTROL_STACK.width + CONTROL_STACK.margin,
      height: CONTROL_STACK.height + top,
    },
    // Bottom-centre: the registrations handle.
    {
      x: size.width / 2 - HANDLE.width / 2,
      y: size.height - bottom - HANDLE.height - HANDLE.margin,
      width: HANDLE.width,
      height: HANDLE.height + bottom + HANDLE.margin,
    },
  ]
}

// A safe-area reading we can add to a pixel extent.
//
// `Math.max(0, NaN)` is `NaN`, which would propagate into a CSS coordinate and place the arrow nowhere — so
// the non-finite case is checked explicitly rather than assumed away by the clamp. Not hypothetical:
// `parseFloat('')` on an unset custom property is exactly `NaN`, and the safe-area values can read as anything
// before the first paint (see `safeArea.ts`).
function usableInset(value: number): number {
  if (!Number.isFinite(value) || value < 0) return 0
  return value
}
