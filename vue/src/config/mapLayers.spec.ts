import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

import {
  DEFAULT_BASE_LAYER,
  MAX_ZOOM,
  MIN_ZOOM,
  TILE_RETRY_BASE_DELAY_MS,
  TILE_RETRY_JITTER_MS,
  TILE_RETRY_LIMIT,
  baseLayers,
  type BaseLayerKey,
} from '@/config/map'

// The shared map configuration (task 353).
//
// # What went wrong, and what these tests are for
//
// The app's map and the public map island (PRD 011 §8) must draw the same layers. They cannot share a module
// — one is TypeScript in a bundle, the other a plain script on a page that loads no bundle — so the island
// carried a hand-copied WMS URL with a comment admitting the duplication. The copy drifted to a **different
// Dataforsyningen service** from any of the app's three, and nothing noticed, because nothing compared them.
//
// The definitions now live in `public/maplayers.json`, which `config/map.ts` imports at build time and the
// island fetches at runtime. These tests hold both ends to it: the file is complete and well-formed, the
// module exposes exactly what it contains, and the island reads it rather than repeating it.

const PUBLIC_DIR = resolve(__dirname, '../../public')
const ISLAND = readFileSync(resolve(PUBLIC_DIR, 'publicmap.js'), 'utf8')
const RAW = readFileSync(resolve(PUBLIC_DIR, 'maplayers.json'), 'utf8')

// The island with its comments stripped, because the assertions below are about what it *does*.
//
// Not pedantry: the island's own comments name the things it must not use — the app's localStorage key, the
// WMS service it used to point at — precisely so the next reader knows why. Asserting on the raw text made
// the explanation indistinguishable from the mistake, and the first version of this spec failed on its own
// documentation. Every comment in that file is a line comment, so one regex is enough.
const CODE = ISLAND.replace(/^\s*\/\/.*$/gm, '')

interface LayerFile {
  attribution: string
  default: string
  minZoom: number
  maxZoom: number
  retry: { limit: number; baseDelayMs: number; jitterMs: number }
  layers: Record<string, { label: string; url: string; layer: string; format: string; note?: string }>
}

const file = JSON.parse(RAW) as LayerFile

describe('the shared map configuration', () => {
  // Not a formality: the island fetches this file over HTTP with no build step in between, so anything
  // JSON.parse rejects — a trailing comma, a // comment — leaves the public map with no layers at all.
  // TypeScript would have tolerated both in the module this replaced.
  it('is valid JSON the island can fetch as-is', () => {
    expect(() => JSON.parse(RAW)).not.toThrow()
  })

  it('carries every layer the app knows about', () => {
    const expected: BaseLayerKey[] = ['dtk25', 'dtk50', 'orto']
    expect(Object.keys(file.layers).sort()).toEqual([...expected].sort())

    // The check the old `satisfies Record<BaseLayerKey, BaseLayerConfig>` gave for free and a JSON import
    // cannot: a key removed from the file would otherwise surface as `undefined` at runtime, on a map, in
    // front of a family.
    for (const key of expected) {
      expect(baseLayers[key], `${key} is missing from the exported layers`).toBeTruthy()
      expect(baseLayers[key].url).toMatch(/^https:\/\/api\.dataforsyningen\.dk\//)
      expect(baseLayers[key].layer.length).toBeGreaterThan(0)
      expect(baseLayers[key].attribution).toBe(file.attribution)
    }
  })

  // ~15× the bytes per tile for aerial imagery as PNG (the file records the measurement). The topographic
  // layers stay PNG because JPEG smears thin contours and place names.
  it('keeps the formats the measurements chose', () => {
    expect(baseLayers.orto.format).toBe('image/jpeg')
    expect(baseLayers.dtk25.format).toBe('image/png')
    expect(baseLayers.dtk50.format).toBe('image/png')
  })

  it('exposes the file’s zoom limits and retry policy rather than its own copies', () => {
    expect(MIN_ZOOM).toBe(file.minZoom)
    expect(MAX_ZOOM).toBe(file.maxZoom)
    expect(DEFAULT_BASE_LAYER).toBe(file.default)
    expect(TILE_RETRY_LIMIT).toBe(file.retry.limit)
    expect(TILE_RETRY_BASE_DELAY_MS).toBe(file.retry.baseDelayMs)
    expect(TILE_RETRY_JITTER_MS).toBe(file.retry.jitterMs)
  })

  // A default naming a layer the file does not contain is the one failure mode that leaves a map with no
  // tiles at all — a blank grey square rather than a missing switcher.
  it('names a default that exists', () => {
    expect(Object.keys(file.layers)).toContain(file.default)
  })
})

describe('the public map island', () => {
  it('reads the shared file instead of hardcoding a service', () => {
    expect(CODE).toContain("fetchJSON('/maplayers.json')")

    // The specific mistake being prevented: the island used to point at `dkskaermkort_DAF`, which is not one
    // of the app's layers. Any Dataforsyningen URL in its code that the shared config does not contain is
    // the same bug returning.
    const urls = CODE.match(/https:\/\/api\.dataforsyningen\.dk\/[A-Za-z0-9_]+/g) ?? []
    const allowed = Object.values(file.layers).map((layer) => layer.url)
    for (const url of urls) {
      expect(allowed, `${url} is not one of the app's layers`).toContain(url)
    }
  })

  // The fallback exists so a failed fetch costs the switcher rather than the map. It must not reintroduce a
  // service the app does not use — which is exactly what the old hardcoded constant was.
  it('has a fallback that is one of the app’s own layers', () => {
    expect(CODE).toContain('FALLBACK_MAP_CONFIG')
    expect(CODE).toContain(baseLayers.dtk25.url)
  })

  // Leaflet has no built-in retry, so this is the whole mechanism: the tileerror event, a bounded per-tile
  // attempt count, backoff with jitter, the cache-busting parameter, and the check that the tile is still in
  // the document.
  it('retries broken tiles the way the app does', () => {
    expect(CODE).toContain("layer.on('tileerror'")
    expect(CODE).toContain('_hejRetries')
    expect(CODE).toContain("'&_retry='")
    expect(CODE).toContain('Math.pow(2, attempt - 1)')
    expect(CODE).toContain('Math.random()')
    expect(CODE).toContain('tile.isConnected')
    // Read from the shared policy, not written in again.
    expect(CODE).toContain('retry.baseDelayMs')
    expect(CODE).toContain('retry.limit')
  })

  // **Solid is “we recorded this”; dotted is “we are joining two things we know”** (task 354). The distinction
  // is the honesty of the drawing, so the dotted legs must be their own stroke rather than more segments in
  // the route — and they must be visibly weaker: dashed, thinner, fainter.
  it('draws the untracked legs as a separate, dotted stroke', () => {
    expect(CODE).toContain('map_.untracked')
    expect(CODE).toContain('dashArray')
    expect(CODE).toContain("lineCap: 'round'")

    // The two polylines must not share their options object: one set of weights for both would let a later
    // tidy-up make the guess look like the record.
    const solid = CODE.indexOf('map_.track && map_.track.length')
    const dotted = CODE.indexOf('map_.untracked && map_.untracked.length')
    expect(solid).toBeGreaterThan(-1)
    expect(dotted).toBeGreaterThan(solid)
    expect(CODE.slice(solid, dotted)).not.toContain('dashArray')
  })

  // The page states a distance in kilometres; a map with no scale next to it cannot be read against that
  // number. Metric only, and bottom left — the one corner the zoom, the switcher and the attribution leave
  // free.
  it('shows a metric scale bar in the free corner', () => {
    expect(CODE).toContain('L.control.scale(')
    expect(CODE).toContain("position: 'bottomleft'")
    expect(CODE).toMatch(/imperial:\s*false/)
  })

  // The app stores the member's choice under `hej.map.baseLayer`. This page is read by people who are not
  // members, and the island holds no state by design.
  it('does not touch the app’s persisted layer choice', () => {
    expect(CODE).not.toContain('hej.map.baseLayer')
    expect(CODE).not.toContain('localStorage')
  })
})
