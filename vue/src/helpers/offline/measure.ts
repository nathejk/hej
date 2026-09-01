// Measuring what is actually stored in the Cache API (task 192).
//
// # Why measurement is approximate, deliberately
//
// The exact size of a cached response can only be had by reading its body. On a full tile cache
// that is several hundred megabytes decoded to produce a number whose only use is a line of text
// in the readiness view — spending the battery of a phone in a forest on precision nobody needs.
//
// So: `Content-Length` when the server sends it, and measured per-zoom fallbacks otherwise
// (Dataforsyningen sends no cache headers at all, so the fallback is the normal path here).
// `navigator.storage.estimate()` remains the honest total for the origin; these figures exist to
// attribute that total to datasets, which the browser will not do for us.

import { estimateResponseBytes, type CacheStorageLike } from '@/helpers/offline/eviction'
import { tileZoomFromUrl } from '@/helpers/offline/tileZoom'

export interface CacheMeasurement {
  itemCount: number
  bytes: number
}

/**
 * Count and approximate the size of one named cache.
 *
 * Returns zeros for a cache that does not exist, rather than throwing: on a first run, before the
 * user has looked at a map, the tile cache genuinely is not there yet, and that is "nothing
 * stored" rather than an error.
 */
export async function measureCache(
  caches: CacheStorageLike | undefined,
  name: string,
): Promise<CacheMeasurement> {
  if (!caches) return { itemCount: 0, bytes: 0 }

  try {
    if (caches.has && !(await caches.has(name))) return { itemCount: 0, bytes: 0 }

    const cache = await caches.open(name)
    const keys = await cache.keys()

    let bytes = 0
    for (const request of keys) {
      const response = await cache.match(request)
      bytes += estimateResponseBytes(response, tileZoomFromUrl(request.url))
    }

    return { itemCount: keys.length, bytes }
  } catch {
    return { itemCount: 0, bytes: 0 }
  }
}

/**
 * Measure the app shell: every cache Workbox created for the precache and its runtime assets.
 *
 * Matched by name prefix rather than by a constant, because the names are Workbox's to choose
 * (`workbox-precache-v2-…`, keyed by scope) and hardcoding one would silently measure nothing
 * after a plugin upgrade. Our own named caches are excluded, since they are separate datasets with
 * their own rows.
 */
export async function measureShell(
  caches: (CacheStorageLike & { keys?: () => Promise<string[]> }) | undefined,
  ours: readonly string[],
): Promise<CacheMeasurement> {
  if (!caches?.keys) return { itemCount: 0, bytes: 0 }

  try {
    const names = (await caches.keys()).filter((name) => !ours.includes(name))
    const measurements = await Promise.all(names.map((name) => measureCache(caches, name)))

    return measurements.reduce(
      (total, m) => ({ itemCount: total.itemCount + m.itemCount, bytes: total.bytes + m.bytes }),
      { itemCount: 0, bytes: 0 },
    )
  } catch {
    return { itemCount: 0, bytes: 0 }
  }
}
