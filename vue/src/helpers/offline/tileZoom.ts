// Working out which zoom a cached map tile belongs to (task 186).
//
// # Why this is not simply reading `z` from the URL
//
// The map uses **WMS**, not an XYZ/WMTS tile scheme (`L.tileLayer.wms` in
// `components/map/EventMap.vue`). A WMS request identifies its tile by a bounding box and a
// pixel size — there is no zoom parameter anywhere in the URL:
//
//   …/dtk_25_DAF?…&bbox=1252344.27,7514065.62,1330615.72,7592337.07&width=256&height=256
//
// So PRD 009's "evict the highest zoom first" cannot be implemented by matching a path segment.
// It has to be *derived*, and the bbox is what carries the information: Leaflet's default CRS is
// EPSG:3857, whose world is 40 075 016.686 m across, and at zoom z that world is 2^z tiles wide.
// One tile therefore spans `40075016.686 / 2^z` metres, which inverts cleanly.
//
// This is worth the derivation rather than switching the map to WMTS: the layer choice is PRD
// 002's and changing it would re-open CORS, the retry logic and the `transparent=false` quirk,
// all of which took measuring to get right.

/** The circumference of the earth at the equator in EPSG:3857 metres — the width of the world. */
const WORLD_METRES = 40_075_016.686

/**
 * Zoom level of a cached tile request, or null when the URL is not a recognisable tile.
 *
 * Rounded rather than floored: floating-point bboxes and a server that snaps to its own grid both
 * put the computed value a hair off an integer, and `Math.floor` would turn z16 into z15 for a
 * whole layer. Anything further than a quarter of a level from an integer is treated as *not a
 * tile* instead of being rounded to the nearest one — better to leave an unrecognised entry alone
 * than to delete it believing it is something it is not.
 */
export function tileZoomFromUrl(url: string): number | null {
  let params: URLSearchParams
  try {
    params = new URL(url).searchParams
  } catch {
    return null
  }

  const bbox = params.get('bbox') ?? params.get('BBOX')
  if (!bbox) return null

  const parts = bbox.split(',').map(Number)
  if (parts.length !== 4 || parts.some((n) => !Number.isFinite(n))) return null

  const [minX, , maxX] = parts
  const span = Math.abs(maxX - minX)
  if (span <= 0) return null

  const zoom = Math.log2(WORLD_METRES / span)
  if (!Number.isFinite(zoom)) return null

  const rounded = Math.round(zoom)
  if (Math.abs(zoom - rounded) > 0.25) return null
  if (rounded < 0 || rounded > 24) return null

  return rounded
}
