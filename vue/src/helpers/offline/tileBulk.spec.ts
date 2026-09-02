import { describe, expect, it } from 'vitest'

import { TILE_TIERS } from '@/config/cache'
import { estimateResponseBytes } from '@/helpers/offline/eviction'
import { BULK_LAYERS, planTiles } from '@/helpers/offline/tileBulk'
import type { RaceArea } from '@/helpers/offline/tileArea'

// Roughly the shape and scale of the real thing: a 20 × 17 km box in North Zealand, ~400 km².
const area: RaceArea = {
  polygon: [
    { latitude: 55.8, longitude: 12.1 },
    { latitude: 55.8, longitude: 12.4 },
    { latitude: 55.95, longitude: 12.4 },
    { latitude: 55.95, longitude: 12.1 },
  ],
  south_west: { latitude: 55.8, longitude: 12.1 },
  north_east: { latitude: 55.95, longitude: 12.4 },
  area_km2: 400,
}

describe('the layers included in a bulk download', () => {
  it('takes the topographic map and the aerial photo', () => {
    expect([...BULK_LAYERS].sort()).toEqual(['dtk25', 'orto'])
  })

  // dtk50 is the same country at half the detail, its own service says it has not been updated since
  // 2017, and nobody navigates a night hike by it. Caching it would add a third again for a fallback.
  it('leaves out the 1:50.000 map', () => {
    expect(BULK_LAYERS).not.toContain('dtk50')
  })
})

describe('planTiles', () => {
  it('plans both layers for every zoom in the tier', () => {
    const plan = planTiles(area, 'orientation')
    const zooms = [...new Set(plan.tiles.map((t) => t.tile.z))].sort()

    expect(zooms).toEqual([...TILE_TIERS.orientation])
    expect(new Set(plan.tiles.map((t) => t.layer))).toEqual(new Set(BULK_LAYERS))
  })

  // The estimate has to exist before the first request: on iOS the app cannot tell WiFi from cellular,
  // so this number is the entire consent mechanism for a download of this size.
  it('estimates bytes without fetching anything', () => {
    const plan = planTiles(area, 'orientation')

    expect(plan.estimatedBytes).toBeGreaterThan(0)
    // Sanity against the measured ~56 MB for the real area's orientation tier: same order of magnitude
    // for an area of comparable size, not an exact match, since this fixture is a box rather than a hull.
    expect(plan.estimatedBytes).toBeGreaterThan(5 * 1024 * 1024)
    expect(plan.estimatedBytes).toBeLessThan(200 * 1024 * 1024)
  })

  // z15–16 is where the bytes are: ~268 MB against ~56 MB for the orientation view. That ratio is the
  // reason the download is tiered at all rather than being one number.
  it('makes the detail tier several times heavier than the orientation tier', () => {
    const orientation = planTiles(area, 'orientation')
    const detail = planTiles(area, 'detail')

    expect(detail.tiles.length).toBeGreaterThan(orientation.tiles.length * 3)
    expect(detail.estimatedBytes).toBeGreaterThan(orientation.estimatedBytes * 2)
  })

  // The aerial layer is JPEG and roughly a tenth of a topo tile at the same zoom. Estimating it as though
  // it were PNG would nearly double the figure shown to the user and make them decline a cheap download.
  it('counts an aerial tile as much cheaper than a topographic one', () => {
    const plan = planTiles(area, 'orientation')

    const topoTiles = plan.tiles.filter((t) => t.layer === 'dtk25')
    const aerialTiles = plan.tiles.filter((t) => t.layer === 'orto')
    expect(topoTiles.length).toBe(aerialTiles.length)

    // What the topographic half alone is estimated at, from the same per-zoom figures.
    const topoBytes = topoTiles.reduce((sum, t) => sum + estimateResponseBytes(undefined, t.tile.z), 0)

    // Adding the aerial layer costs a fraction on top, not another whole map.
    expect(plan.estimatedBytes).toBeGreaterThan(topoBytes)
    expect(plan.estimatedBytes).toBeLessThan(topoBytes * 1.25)
  })
})
