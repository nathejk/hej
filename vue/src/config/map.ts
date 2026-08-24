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
  /** Extra note surfaced in the switcher, e.g. data currency caveats. */
  note?: string
}

// The Dataforsyningen token is a public quota key for a public service, not a
// credential — but it is still not committed. Set VITE_DATAFORSYNINGEN_TOKEN in
// docker-compose.override.yml for dev and in the deploy environment for prod.
// When it is missing the map reports it instead of silently showing grey tiles.
export const DATAFORSYNINGEN_TOKEN = import.meta.env.VITE_DATAFORSYNINGEN_TOKEN ?? ''

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
  },
  dtk50: {
    label: 'Topografisk 1:50.000',
    url: 'https://api.dataforsyningen.dk/dtk_50_DAF',
    layer: 'dtk_50',
    attribution: DATAFORSYNINGEN_ATTRIBUTION,
    // The service itself states it is not updated after 2017.
    note: 'Kortdata fra 2017',
  },
  orto: {
    label: 'Luftfoto',
    url: 'https://api.dataforsyningen.dk/orto_foraar_DAF',
    layer: 'orto_foraar',
    attribution: DATAFORSYNINGEN_ATTRIBUTION,
  },
} as const satisfies Record<BaseLayerKey, BaseLayerConfig>

export const DEFAULT_BASE_LAYER: BaseLayerKey = 'dtk25'

/** localStorage key for the user's base-layer choice. */
export const BASE_LAYER_STORAGE_KEY = 'hej.map.baseLayer'

// Opening view used until we have a position. Central Jutland — replace with the
// actual event area per year (PRD 002 §11 open question).
export const DEFAULT_CENTER: [number, number] = [56.18, 9.47]
export const DEFAULT_ZOOM = 11
export const MIN_ZOOM = 7
export const MAX_ZOOM = 19
/** Zoom used when recentring on the user's own position. */
export const LOCATE_ZOOM = 15
