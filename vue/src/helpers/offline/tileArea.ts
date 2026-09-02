// Working out which tiles the race area needs (task 087, PRD 002 §11.2).
//
// # Why the polygon and not just its bounding box
//
// `GET /api/race-area` hands us both, and its own comment suggests iterating the box "which is what a
// tile walk actually iterates". The box is the wrong choice here and the difference is measured in
// hundreds of megabytes: the hull of this year's checkpoints plus a 3 km buffer is ~428 km², while its
// bounding box is a rectangle drawn around a shape that is nowhere near rectangular. Every tile in the
// difference is a tile downloaded over a participant's mobile connection, stored against a quota that
// competes with their portraits, showing land nobody in the race will walk on.
//
// PRD 009's whole budget — 324 MB at z12–16 — was measured for the *area*, not the box. Iterating the
// box would quietly break that number.
//
// So: walk the box, keep the tiles that touch the polygon. That is a handful of lines of geometry, and
// it is the difference between the size the budget was planned for and something considerably larger.
//
// # Why intersection rather than "is the centre inside"
//
// A centre test is right at z16, where a tile is 350 m across, and badly wrong at z12, where it is
// 5.5 km: an entire edge of the race area would be missing, and the participant who discovers it is the
// one standing at the edge with no signal. Intersection costs a little more arithmetic and gets the
// boundary right at every zoom.

/** WGS84 degrees, matching the BFF's `Point`. */
export interface LatLng {
  latitude: number
  longitude: number
}

export interface TileCoord {
  x: number
  y: number
  z: number
}

/** Slippy-map tile x for a longitude at a zoom. */
export function lonToTileX(lon: number, z: number): number {
  return Math.floor(((lon + 180) / 360) * 2 ** z)
}

/** Slippy-map tile y for a latitude at a zoom (Web Mercator). */
export function latToTileY(lat: number, z: number): number {
  const rad = (lat * Math.PI) / 180
  return Math.floor(
    ((1 - Math.log(Math.tan(rad) + 1 / Math.cos(rad)) / Math.PI) / 2) * 2 ** z,
  )
}

/** West/east longitudes and north/south latitudes of a tile. */
export function tileBounds(tile: TileCoord): {
  west: number
  east: number
  north: number
  south: number
} {
  const n = 2 ** tile.z
  const lon = (x: number) => (x / n) * 360 - 180
  const lat = (y: number) => {
    const t = Math.PI * (1 - (2 * y) / n)
    return (180 / Math.PI) * Math.atan(Math.sinh(t))
  }
  return {
    west: lon(tile.x),
    east: lon(tile.x + 1),
    north: lat(tile.y),
    south: lat(tile.y + 1),
  }
}

/**
 * Ray casting, in lon/lat.
 *
 * Fine for this: the polygon is a convex hull of at most a few dozen points, spanning tens of
 * kilometres in Denmark, so neither the antimeridian nor a pole is reachable and the flat-earth
 * simplification costs nothing a 3 km buffer does not already absorb.
 */
export function pointInPolygon(point: LatLng, polygon: readonly LatLng[]): boolean {
  let inside = false
  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    const xi = polygon[i].longitude
    const yi = polygon[i].latitude
    const xj = polygon[j].longitude
    const yj = polygon[j].latitude

    const crosses =
      yi > point.latitude !== yj > point.latitude &&
      point.longitude < ((xj - xi) * (point.latitude - yi)) / (yj - yi) + xi
    if (crosses) inside = !inside
  }
  return inside
}

/**
 * Does this tile touch the polygon at all?
 *
 * Three cases, and all three are needed. A tile can overlap a polygon while containing none of its
 * vertices (a small tile inside a big hull), and a polygon vertex can sit inside a tile that has no
 * corner inside the polygon (a big tile over a narrow spur). The edge-crossing test catches what is
 * left: a tile straddling a boundary with no corner or vertex on the wrong side.
 */
export function tileIntersectsPolygon(tile: TileCoord, polygon: readonly LatLng[]): boolean {
  const b = tileBounds(tile)

  const corners: LatLng[] = [
    { latitude: b.north, longitude: b.west },
    { latitude: b.north, longitude: b.east },
    { latitude: b.south, longitude: b.east },
    { latitude: b.south, longitude: b.west },
  ]
  if (corners.some((c) => pointInPolygon(c, polygon))) return true
  if (
    polygon.some(
      (p) =>
        p.longitude >= b.west &&
        p.longitude <= b.east &&
        p.latitude >= b.south &&
        p.latitude <= b.north,
    )
  ) {
    return true
  }

  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    for (let c = 0; c < 4; c++) {
      if (segmentsCross(polygon[j], polygon[i], corners[c], corners[(c + 1) % 4])) return true
    }
  }
  return false
}

function segmentsCross(a: LatLng, b: LatLng, c: LatLng, d: LatLng): boolean {
  const cross = (p: LatLng, q: LatLng, r: LatLng) =>
    (q.longitude - p.longitude) * (r.latitude - p.latitude) -
    (q.latitude - p.latitude) * (r.longitude - p.longitude)

  const d1 = cross(c, d, a)
  const d2 = cross(c, d, b)
  const d3 = cross(a, b, c)
  const d4 = cross(a, b, d)

  return ((d1 > 0) !== (d2 > 0) || d1 === 0 || d2 === 0) &&
    ((d3 > 0) !== (d4 > 0) || d3 === 0 || d4 === 0)
}

export interface RaceArea {
  polygon: LatLng[]
  south_west: LatLng
  north_east: LatLng
  area_km2: number
}

/**
 * Every tile at the given zooms whose extent touches the race area.
 *
 * Ordered **shallowest zoom first**, and that ordering is a feature rather than an accident: an
 * interrupted download then leaves the orientation view complete rather than a patchwork of detail with
 * no context. z12–14 is 56 MB and is what a lost participant actually needs; z15–16 is 268 MB of
 * refinement on top.
 */
export function tilesForArea(area: RaceArea, zooms: readonly number[]): TileCoord[] {
  const tiles: TileCoord[] = []

  for (const z of [...zooms].sort((a, b) => a - b)) {
    const xMin = lonToTileX(area.south_west.longitude, z)
    const xMax = lonToTileX(area.north_east.longitude, z)
    // y runs north to south, so the north-east corner gives the smaller index.
    const yMin = latToTileY(area.north_east.latitude, z)
    const yMax = latToTileY(area.south_west.latitude, z)

    for (let x = xMin; x <= xMax; x++) {
      for (let y = yMin; y <= yMax; y++) {
        const tile = { x, y, z }
        if (tileIntersectsPolygon(tile, area.polygon)) tiles.push(tile)
      }
    }
  }

  return tiles
}
