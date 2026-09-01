// Reclaiming space when the origin runs out (PRD 009 §6, task 186).
//
// # The rule, and why it is worth stating twice
//
// `QuotaExceededError` is **expected, not exceptional**. A write that fails must leave everything
// already stored intact and working: a full cache that cannot grow is far better than an empty
// one, because offline a discarded tile cannot be re-fetched and a discarded track never existed.
// That is why nothing here — and nothing in `vite.config.ts` — uses Workbox's
// `purgeOnQuotaError`, which deletes an entire cache on a single failed write.
//
// # Eviction has no registry to enforce it
//
// PRD 009 cut the dataset registry (§4, §11.2), so this module acts on caches it does not own,
// through the narrowest adapter each storage kind allows. That is the honest cost of the cut, and
// it is why `config/offline.ts` carries the invariant test and task 195 carries a
// quota-exhaustion test: the order is a declaration, and these are what make it real.
//
// # What it deliberately does not do
//
// No background sweeping, no size watching, no timers. Eviction runs when something actually
// needs room — a failed write — because guessing at "nearly full" from
// `navigator.storage.estimate()` on iOS means acting on a number the browser rounds and pads.

import { TILE_CACHE_NAME } from '@/config/cache'
import {
  TILE_EVICTION_ZOOM_ORDER,
  evictionOrder,
  type OfflineDataset,
  type OfflineDatasetId,
} from '@/config/offline'
import { tileZoomFromUrl } from '@/helpers/offline/tileZoom'

/** True for the error browsers raise when a write does not fit. */
export function isQuotaExceeded(err: unknown): boolean {
  if (!(err instanceof Error)) return false
  // Safari and Firefox report `QuotaExceededError`; older WebKit uses the numeric legacy code 22
  // via `QUOTA_EXCEEDED_ERR`, and Chrome's localStorage path throws a plain DOMException whose
  // name is the same. Checking the name covers all of them without a `DOMException` reference,
  // which does not exist in the node test environment.
  return err.name === 'QuotaExceededError' || err.name === 'QUOTA_EXCEEDED_ERR'
}

/**
 * How much a single eviction pass tries to free, in bytes.
 *
 * Deliberately much larger than one failed write. Freeing exactly enough would mean an eviction
 * on every subsequent tile, each iterating the cache — hundreds of passes while a participant
 * pans a map. 20 MB is roughly 500 tiles at z16 and buys a long stretch of writing.
 */
export const EVICTION_TARGET_BYTES = 20 * 1024 * 1024

/**
 * What eviction is allowed to do to one storage kind.
 *
 * An adapter per dataset rather than one clever mechanism, because the three kinds have nothing
 * in common: a Cache API bucket can be enumerated and deleted entry by entry, `localStorage` is a
 * single string per dataset that can only be dropped whole, and IndexedDB is not evictable here at
 * all. Modelling that honestly beats pretending they are uniform.
 */
export interface DatasetEvictor {
  /**
   * Free up to `targetBytes`, returning what was actually freed.
   *
   * May return 0 — a dataset that is already empty is not an error.
   */
  evict: (targetBytes: number) => Promise<number>
}

export type EvictorMap = Partial<Record<OfflineDatasetId, DatasetEvictor>>

export interface EvictionResult {
  freedBytes: number
  /** Which datasets gave up space, in the order they were asked. */
  evicted: OfflineDatasetId[]
}

/**
 * Walk the priority order from the bottom, freeing space until the target is met.
 *
 * `evictionOrder()` has already removed everything `unrecoverable`, so the track cannot be
 * reached from here even by a caller that keeps asking. That belt-and-braces is deliberate: this
 * is the one function in the app whose bug would be silent data loss.
 */
export async function reclaim(
  evictors: EvictorMap,
  targetBytes: number = EVICTION_TARGET_BYTES,
): Promise<EvictionResult> {
  const result: EvictionResult = { freedBytes: 0, evicted: [] }

  for (const dataset of evictionOrder()) {
    if (result.freedBytes >= targetBytes) break

    const evictor = evictors[dataset.id]
    if (!evictor) continue

    try {
      const freed = await evictor.evict(targetBytes - result.freedBytes)
      if (freed > 0) {
        result.freedBytes += freed
        result.evicted.push(dataset.id)
      }
    } catch {
      // A failing evictor must not stop the ones below it. The most likely cause is the storage
      // being unavailable, in which case there was nothing to reclaim there anyway.
    }
  }

  return result
}

/** The subset of the Cache API used here, injected so the spec can run in node. */
export interface CacheLike {
  keys: () => Promise<readonly Request[]>
  match: (request: Request) => Promise<Response | undefined>
  delete: (request: Request) => Promise<boolean>
}

export interface CacheStorageLike {
  open: (name: string) => Promise<CacheLike>
  has?: (name: string) => Promise<boolean>
}

/**
 * Approximate the stored size of a cached response.
 *
 * `Content-Length` when the service sends it, and a per-zoom fallback when it does not.
 * Dataforsyningen sends no cache headers at all, so the fallback is not a corner case — the
 * numbers come from measuring the live service inside the race area (task 087): topo tiles run
 * 140 kB at z12 down to 32 kB at z16, aerial JPEG around 10 kB.
 *
 * Approximate on purpose. The alternative is reading every body to count bytes, which on a
 * 5,000-tile cache means decoding several hundred megabytes to decide what to delete — spending
 * the battery of a phone in a forest to be precise about a number only used for "have I freed
 * enough yet".
 */
export function estimateResponseBytes(response: Response | undefined, zoom: number | null): number {
  const declared = Number(response?.headers?.get?.('content-length') ?? NaN)
  if (Number.isFinite(declared) && declared > 0) return declared

  if (zoom === null) return 40 * 1024
  if (zoom <= 12) return 140 * 1024
  if (zoom === 13) return 120 * 1024
  if (zoom === 14) return 99 * 1024
  if (zoom === 15) return 60 * 1024
  return 32 * 1024
}

/**
 * Evictor for the map tile cache: **highest zoom first**.
 *
 * Not arbitrary. z16 is ~60% of the tile bytes, and z12–14 is the orientation view a lost
 * participant actually needs — 56 MB for the whole race area against 268 MB for z15–16. Dropping
 * from the top degrades the map gradually rather than punching holes in the middle of it.
 *
 * Tiles whose zoom cannot be determined are evicted **last**, after every known zoom. An
 * unrecognised entry is more likely to be something this code does not understand than a tile we
 * meant to discard, and deleting it first would make a URL-shape change look like working
 * eviction while quietly destroying the cache.
 */
export function tileEvictor(
  caches: CacheStorageLike,
  cacheName: string = TILE_CACHE_NAME,
): DatasetEvictor {
  return {
    async evict(targetBytes) {
      const cache = await caches.open(cacheName)
      const keys = await cache.keys()

      const byZoom = new Map<number, Request[]>()
      const unknown: Request[] = []
      for (const request of keys) {
        const zoom = tileZoomFromUrl(request.url)
        if (zoom === null) {
          unknown.push(request)
          continue
        }
        const bucket = byZoom.get(zoom)
        if (bucket) bucket.push(request)
        else byZoom.set(zoom, [request])
      }

      // Any zoom present in the cache but absent from the declared order still has to be
      // reachable, or a stray z17 would be undeletable. Declared order first, then whatever else
      // exists, deepest first.
      const extraZooms = [...byZoom.keys()]
        .filter((z) => !TILE_EVICTION_ZOOM_ORDER.includes(z))
        .sort((a, b) => b - a)

      let freed = 0
      for (const zoom of [...extraZooms, ...TILE_EVICTION_ZOOM_ORDER]) {
        for (const request of byZoom.get(zoom) ?? []) {
          if (freed >= targetBytes) return freed
          const response = await cache.match(request)
          const bytes = estimateResponseBytes(response, zoom)
          if (await cache.delete(request)) freed += bytes
        }
      }

      for (const request of unknown) {
        if (freed >= targetBytes) return freed
        const response = await cache.match(request)
        const bytes = estimateResponseBytes(response, null)
        if (await cache.delete(request)) freed += bytes
      }

      return freed
    },
  }
}

/**
 * Evictor for a dataset stored as a single value that can only be dropped whole — the contacts
 * directory in `localStorage`.
 *
 * All-or-nothing, so it is offered only when the *whole* value is worth less than the space it
 * frees. `sizeBytes` is measured before deleting so the caller learns what it actually gained.
 */
export function wholeValueEvictor(
  read: () => string | null,
  clear: () => void,
): DatasetEvictor {
  return {
    async evict() {
      const value = read()
      if (!value) return 0
      // UTF-16 in localStorage: two bytes per code unit. Being right about this matters less than
      // not pretending a 1 MB directory freed 500 kB.
      const bytes = value.length * 2
      clear()
      return bytes
    },
  }
}
