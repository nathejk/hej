// Downloading the race area's tiles on request (task 087, PRD 002 §11.2).
//
// This is the half of tile caching that needs a user's consent: hundreds of megabytes, deliberately, on
// a connection the app cannot measure — `navigator.connection` is unavailable in Safari, so there is no
// "only on WiFi" to hide behind. The other half already works and costs nothing (tiles are stored as the
// map is browsed) and is deliberately *not* gated on anything.
//
// # Why the whole area rather than a radius around the user
//
// The scope moved twice before landing here, and the reasoning still applies if the area grows: a
// follow-me radius can only ever be filled where the network already works, which is precisely where the
// cache is not needed. A fixed area is twice the bytes and removes eviction, incomplete coverage, and the
// question "is the map ready?" having no true answer.
//
// # Why the aerial layer is included
//
// The topographic map is what people navigate by, but the aerial layer is what settles an argument about
// which building or clearing is meant, and it is cheap: ~11 kB per tile against ~99 kB for topo at the
// same zoom, because it is JPEG. Skipping it would save little and lose the layer entirely offline.

import { TILE_CACHE_NAME, TILE_TIERS, type TileTier } from '@/config/cache'
import { DEFAULT_BASE_LAYER, type BaseLayerKey } from '@/config/map'
import { dataforsyningenToken } from '@/config/runtime'
import { HttpError, fetchWrapper } from '@/helpers'
import { estimateResponseBytes, type CacheStorageLike } from '@/helpers/offline/eviction'
import { tilesForArea, type RaceArea, type TileCoord } from '@/helpers/offline/tileArea'
import { tileUrlSource } from '@/helpers/offline/tileUrls'
import { downloadTiles, type TileDownloadResult } from '@/helpers/offline/tileDownload'

/**
 * Layers included in a bulk download.
 *
 * The topographic 1:25.000 map and the aerial photo. **`dtk50` is deliberately absent**: it is the same
 * country at half the detail, its own service says it has not been updated since 2017, and nobody
 * navigates a night hike by it. Caching it would add a third of the bytes again for a layer that exists
 * as a fallback.
 */
export const BULK_LAYERS: readonly BaseLayerKey[] = [DEFAULT_BASE_LAYER, 'orto']

/**
 * The three outcomes of asking for the race area, kept apart because the user's situation differs.
 *
 * They were collapsed into `null` at first, with a comment claiming it was "not an error the user can act
 * on". That was wrong in the same way task 197 was wrong: a participant taps a button, the label flickers
 * for half a second, and nothing happens or is said. Reported on a real phone, 2026-09-02.
 */
export type RaceAreaResult =
  /** The area, ready to download. */
  | { kind: 'area'; area: RaceArea }
  /** The server has no area yet — 404. Normal early in the year, before checkpoints have positions. */
  | { kind: 'none' }
  /** We could not ask. No signal, or the request failed. */
  | { kind: 'offline' }

export async function fetchRaceArea(): Promise<RaceAreaResult> {
  try {
    return { kind: 'area', area: await fetchWrapper.get<RaceArea>('/api/race-area') }
  } catch (err) {
    // 404 is a deliberate answer rather than a failure: the endpoint returns it when no area can be
    // derived, precisely so a client cannot mistake "nothing" for "cache everything" and try to download
    // the whole country. It means *not yet*, which is a different sentence from *no signal*.
    if (err instanceof HttpError && err.status === 404) return { kind: 'none' }
    return { kind: 'offline' }
  }
}

export interface TilePlan {
  tiles: { layer: BaseLayerKey; tile: TileCoord }[]
  /** Estimated bytes, from tile sizes measured against the live service (task 087). */
  estimatedBytes: number
}

/**
 * What a tier's download would involve, before fetching anything.
 *
 * The estimate has to exist *before* the first request, because on iOS the app cannot tell WiFi from
 * cellular and the number on screen is therefore the whole consent mechanism. It is deliberately built
 * from per-zoom measurements rather than from a content-length nobody has asked for yet.
 */
export function planTiles(area: RaceArea, tier: TileTier): TilePlan {
  const zooms = TILE_TIERS[tier]
  const tiles: TilePlan['tiles'] = []
  let estimatedBytes = 0

  for (const layer of BULK_LAYERS) {
    for (const tile of tilesForArea(area, zooms)) {
      tiles.push({ layer, tile })
      // The aerial layer is JPEG and roughly a tenth of a topo tile at the same zoom; the per-zoom topo
      // figures come from measuring inside the real race area, where cartography is denser than the
      // rural sample first used.
      estimatedBytes +=
        layer === 'orto'
          ? Math.round(estimateResponseBytes(undefined, tile.z) * 0.11)
          : estimateResponseBytes(undefined, tile.z)
    }
  }

  return { tiles, estimatedBytes }
}

export interface BulkDownloadOptions {
  area: RaceArea
  tier: TileTier
  caches: CacheStorageLike
  signal?: AbortSignal
  onProgress?: (done: number, total: number) => void
}

/**
 * Fetch one tier of the race area.
 *
 * Layers are done one after another and tiles shallowest-zoom-first (see `tilesForArea`), so an
 * interruption leaves something coherent rather than a patchwork: the orientation view of the topo map
 * survives before any aerial detail is attempted.
 */
export async function downloadRaceArea(
  options: BulkDownloadOptions,
): Promise<TileDownloadResult> {
  const plan = planTiles(options.area, options.tier)
  const token = dataforsyningenToken.value

  // Built once per layer and disposed, because each holds a detached Leaflet map.
  const urls: string[] = []
  for (const layer of BULK_LAYERS) {
    const source = await tileUrlSource(layer, token)
    try {
      for (const entry of plan.tiles) {
        if (entry.layer === layer) urls.push(source.url(entry.tile))
      }
    } finally {
      source.dispose()
    }
  }

  return downloadTiles({
    urls,
    cacheName: TILE_CACHE_NAME,
    caches: options.caches,
    fetch: (url) =>
      // `no-cors` would give an opaque response, which browsers pad heavily for quota accounting — the
      // same trap the map avoids with `crossOrigin: 'anonymous'`. Dataforsyningen reflects
      // `Access-Control-Allow-Origin`, so a readable cross-origin response is available and is what the
      // service worker stores for the map.
      fetch(url, { mode: 'cors', credentials: 'omit' }),
    signal: options.signal,
    onProgress: (p) => options.onProgress?.(p.done, p.total),
  })
}
