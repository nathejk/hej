// Map configuration: base layers, default view and zoom limits.
//
// All three base layers are Dataforsyningen WMS services. The service paths and
// WMS layer names below were verified against live GetCapabilities + GetMap
// responses on 2026-08-24 — do NOT infer them by analogy: `dtk_25_DAF` answers
// to `DTK25`/`dtk25`/`dtk_25`, but `dtk_50_DAF` rejects `DTK50` and only accepts
// `dtk_50`. See PRD 002 §11.

export interface BaseLayerConfig {
  /** Label shown in the layer switcher (Danish). */
  label: string
  /** WMS endpoint. */
  url: string
  /** WMS layer name. */
  layer: string
  attribution: string
  /**
   * WMS output format.
   *
   * Per layer, not global, because the right answer differs by content type and the
   * difference is large. Measured against the live service (256 px tiles, central
   * Zealand, 2026-08-26):
   *
   *   orto_foraar   PNG ~137 kB/tile   JPEG ~9-14 kB/tile   (~15x)
   *
   * PNG stores photographic detail losslessly, which is exactly the wrong trade for
   * aerial imagery — and this app is used on rural mobile data, so it is a real cost
   * rather than a theoretical one. It also matters for PRD 009's offline budget, where
   * tiles are the largest cached dataset.
   *
   * The topographic layers stay PNG deliberately: they are line art and text, where
   * JPEG's block artefacts smear thin contours and place names.
   */
  format: 'image/png' | 'image/jpeg'
  /** Extra note surfaced in the switcher, e.g. data currency caveats. */
  note?: string
}

// The Dataforsyningen token is not inlined here: it is fetched at runtime from
// GET /api/config (see config/runtime.ts) so the same built image can be
// deployed with a different key. When it is missing the map reports it instead
// of silently showing grey tiles.

const DATAFORSYNINGEN_ATTRIBUTION =
  '&copy; <a target="_blank" rel="noopener" href="https://dataforsyningen.dk/">Styrelsen for Dataforsyning og Infrastruktur</a>'

// Keys are stable identifiers persisted in localStorage — renaming one resets
// the user's layer choice, so don't. Typed as a Record so every entry exposes the
// full config shape (including the optional `note`) rather than being narrowed to
// its own literal.
export type BaseLayerKey = 'dtk25' | 'dtk50' | 'orto'

export const baseLayers: Record<BaseLayerKey, BaseLayerConfig> = {
  dtk25: {
    label: 'Topografisk 1:25.000',
    url: 'https://api.dataforsyningen.dk/dtk_25_DAF',
    layer: 'dtk25',
    attribution: DATAFORSYNINGEN_ATTRIBUTION,
    format: 'image/png',
  },
  dtk50: {
    label: 'Topografisk 1:50.000',
    url: 'https://api.dataforsyningen.dk/dtk_50_DAF',
    layer: 'dtk_50',
    attribution: DATAFORSYNINGEN_ATTRIBUTION,
    format: 'image/png',
    // The service itself states it is not updated after 2017.
    note: 'Kortdata fra 2017',
  },
  orto: {
    label: 'Luftfoto',
    url: 'https://api.dataforsyningen.dk/orto_foraar_DAF',
    layer: 'orto_foraar',
    attribution: DATAFORSYNINGEN_ATTRIBUTION,
    format: 'image/jpeg',
  },
} as const satisfies Record<BaseLayerKey, BaseLayerConfig>

export const DEFAULT_BASE_LAYER: BaseLayerKey = 'dtk25'

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
// position is available.
export const MIN_ZOOM = 7
export const MAX_ZOOM = 19
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

// Tile retry policy. Leaflet has no built-in retry: a single failed image request
// leaves that tile grey until the user pans away and back. On patchy rural mobile
// data that is the normal case, not the exception, so failed tiles are retried
// with backoff before we admit defeat.
export const TILE_RETRY_LIMIT = 3
export const TILE_RETRY_BASE_DELAY_MS = 400

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
