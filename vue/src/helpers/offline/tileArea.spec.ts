import { describe, expect, it } from 'vitest'

import {
  latToTileY,
  lonToTileX,
  pointInPolygon,
  tileBounds,
  tileIntersectsPolygon,
  tilesForArea,
  type LatLng,
  type RaceArea,
} from '@/helpers/offline/tileArea'

// A square roughly 20 km across in North Zealand, which is where the real race area is.
const square: LatLng[] = [
  { latitude: 55.8, longitude: 12.1 },
  { latitude: 55.8, longitude: 12.4 },
  { latitude: 55.95, longitude: 12.4 },
  { latitude: 55.95, longitude: 12.1 },
]

function areaOf(polygon: LatLng[]): RaceArea {
  const lats = polygon.map((p) => p.latitude)
  const lons = polygon.map((p) => p.longitude)
  return {
    polygon,
    south_west: { latitude: Math.min(...lats), longitude: Math.min(...lons) },
    north_east: { latitude: Math.max(...lats), longitude: Math.max(...lons) },
    area_km2: 400,
  }
}

describe('tile coordinates', () => {
  // Checked against the slippy-map formulas rather than against our own output:
  // x = floor((12.5683 + 180) / 360 × 2^12) = floor(2190.99) = 2190, and
  // y = floor((1 − asinh(tan 55.6761°)/π) / 2 × 2^12) = floor(1282.03) = 1282.
  it('places Copenhagen in the expected tile at z12', () => {
    expect(lonToTileX(12.5683, 12)).toBe(2190)
    expect(latToTileY(55.6761, 12)).toBe(1282)
  })

  it('round-trips a tile through its bounds', () => {
    const tile = { x: 2190, y: 1282, z: 12 }
    const b = tileBounds(tile)

    expect(b.west).toBeLessThan(b.east)
    expect(b.south).toBeLessThan(b.north)
    // A point just inside the tile must map back to it.
    expect(lonToTileX((b.west + b.east) / 2, 12)).toBe(tile.x)
    expect(latToTileY((b.north + b.south) / 2, 12)).toBe(tile.y)
  })

  it('gives a z12 tile roughly 5.5 km of longitude', () => {
    const b = tileBounds({ x: 2190, y: 1282, z: 12 })
    const km = (b.east - b.west) * 111.32 * Math.cos((55.7 * Math.PI) / 180)
    expect(km).toBeGreaterThan(4)
    expect(km).toBeLessThan(7)
  })
})

describe('pointInPolygon', () => {
  it('accepts a point inside and rejects one outside', () => {
    expect(pointInPolygon({ latitude: 55.87, longitude: 12.25 }, square)).toBe(true)
    expect(pointInPolygon({ latitude: 55.6, longitude: 12.25 }, square)).toBe(false)
    expect(pointInPolygon({ latitude: 55.87, longitude: 11.0 }, square)).toBe(false)
  })
})

describe('tileIntersectsPolygon', () => {
  it('accepts a tile wholly inside the area', () => {
    const z = 16
    const tile = { x: lonToTileX(12.25, z), y: latToTileY(55.87, z), z }
    expect(tileIntersectsPolygon(tile, square)).toBe(true)
  })

  it('rejects a tile far outside', () => {
    const z = 16
    const tile = { x: lonToTileX(9.5, z), y: latToTileY(57.0, z), z }
    expect(tileIntersectsPolygon(tile, square)).toBe(false)
  })

  // The case a centre test gets wrong, and the reason this is an intersection test: at z12 a tile is
  // 5.5 km across, so a tile can cover the edge of the race area while its centre sits outside it. Miss
  // those and the participant who finds out is the one standing at the edge with no signal.
  it('accepts a big tile that straddles the boundary', () => {
    const z = 12
    // A tile containing the south-west corner: most of it is outside the square.
    const tile = { x: lonToTileX(12.1, z), y: latToTileY(55.8, z), z }
    const b = tileBounds(tile)

    // Confirm the fixture really is the awkward case rather than an accidentally-inside tile.
    const centre = {
      latitude: (b.north + b.south) / 2,
      longitude: (b.west + b.east) / 2,
    }
    expect(pointInPolygon(centre, square)).toBe(false)
    expect(tileIntersectsPolygon(tile, square)).toBe(true)
  })

  // A polygon vertex inside a tile that has no corner inside the polygon — a big tile over a narrow
  // spur of the hull. Neither the corner test nor the edge test alone catches it.
  it('accepts a tile that contains only a polygon vertex', () => {
    const spur: LatLng[] = [
      { latitude: 55.8, longitude: 12.1 },
      { latitude: 55.8005, longitude: 12.1005 },
      { latitude: 55.8, longitude: 12.101 },
    ]
    const z = 12
    const tile = { x: lonToTileX(12.1005, z), y: latToTileY(55.8003, z), z }

    expect(tileIntersectsPolygon(tile, spur)).toBe(true)
  })
})

describe('tilesForArea', () => {
  it('returns tiles for every requested zoom, shallowest first', () => {
    const tiles = tilesForArea(areaOf(square), [14, 12, 13])
    const zooms = [...new Set(tiles.map((t) => t.z))]

    // Ordering is a feature: an interrupted download then leaves the orientation view complete rather
    // than detail with no context.
    expect(zooms).toEqual([12, 13, 14])
  })

  it('grows roughly fourfold per zoom level', () => {
    const area = areaOf(square)
    const z14 = tilesForArea(area, [14]).length
    const z15 = tilesForArea(area, [15]).length

    expect(z15 / z14).toBeGreaterThan(3)
    expect(z15 / z14).toBeLessThan(5)
  })

  // The whole reason for the polygon test. A hull is not a rectangle, and every tile in the difference
  // is a tile downloaded over mobile data, stored against a quota that competes with portraits, showing
  // land nobody in the race will walk on. PRD 009's 324 MB was measured for the area, not the box.
  it('caches fewer tiles than the bounding box for a non-rectangular area', () => {
    const triangle: LatLng[] = [
      { latitude: 55.8, longitude: 12.1 },
      { latitude: 55.8, longitude: 12.4 },
      { latitude: 55.95, longitude: 12.4 },
    ]
    const area = areaOf(triangle)

    const forTriangle = tilesForArea(area, [14]).length
    const forBox = tilesForArea({ ...area, polygon: square }, [14]).length

    expect(forTriangle).toBeLessThan(forBox * 0.8)
    // But not so aggressive that the triangle loses its own edges.
    expect(forTriangle).toBeGreaterThan(forBox * 0.4)
  })

  it('returns nothing for no zooms rather than everything', () => {
    expect(tilesForArea(areaOf(square), [])).toEqual([])
  })
})
