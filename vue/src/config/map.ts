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

// Where an edge arrow may not go (PRD 016, task 264).
//
// `MapsView` floats three things over the map: the layer/locate control stack top-right, the notices top-left,
// and the registrations handle bottom-centre — all in the same overlay layer the arrows use. An arrow that
// lands under the locate button is worse than no arrow at all, because it is invisible *and* it steals the
// tap.
//
// So arrows are confined to the **vertical middle band** of each edge. The extents below are what the controls
// themselves occupy, in CSS pixels, *excluding* the safe-area inset — which is added at call time from the
// same `--sat`/`--sab` custom properties the controls are positioned with (`arrowKeepOut`). Deriving it rather
// than hardcoding one number matters on a notched phone: `--sat` is 59 px on the maintainer's iPhone, so a
// fixed keep-out tuned on a desktop browser would put arrows under the layer switcher on every real device.
export const CONTROL_EXTENT = {
  /** The control stack (two ≥ 44 px targets plus spacing) and the notices that sit level with it. */
  top: 112,
  /** The registrations handle plus the bottom nav. */
  bottom: 96,
  /** Nothing floats at the sides — just enough to clear the screen edge. */
  sides: 8,
} as const

/** Margins an arrow must stay out of. */
export interface ArrowInsets {
  top: number
  right: number
  bottom: number
  left: number
}

/**
 * The keep-out margins for a given safe-area reading.
 *
 * Pure, so the composition can be tested without a browser — the reading needs a device, the arithmetic does
 * not. Same split as `safeArea.insetVars`.
 *
 * A negative or non-finite reading is treated as zero rather than trusted: the safe-area properties can read
 * as anything before the first paint (see `safeArea.ts`, which discards an all-zero read for the same class of
 * reason), and either value here would place an arrow off screen.
 */
export function arrowKeepOut(safe: { top: number; bottom: number }): ArrowInsets {
  return {
    top: CONTROL_EXTENT.top + usableInset(safe.top),
    bottom: CONTROL_EXTENT.bottom + usableInset(safe.bottom),
    right: CONTROL_EXTENT.sides,
    left: CONTROL_EXTENT.sides,
  }
}

// A safe-area reading we can add to a pixel extent.
//
// `Math.max(0, NaN)` is `NaN`, which would propagate into a CSS `top` and place the arrow nowhere — so the
// non-finite case is checked explicitly rather than assumed away by the clamp. Not hypothetical:
// `parseFloat('')` on an unset custom property is exactly `NaN`, and this function is public.
function usableInset(value: number): number {
  if (!Number.isFinite(value) || value < 0) return 0
  return value
}
