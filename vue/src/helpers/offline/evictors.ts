// Evictors bound to the real browser caches (task 192).
//
// Kept out of `reporters.ts` so nothing that imports an evictor imports a Pinia store: the contacts
// store needs these to make room for a write, and `reporters.ts` needs the contacts store, which
// would be a cycle through module initialisation for no reason.

import { PORTRAIT_CACHE_NAME, TILE_CACHE_NAME } from '@/config/cache'
import { tileEvictor, type CacheStorageLike, type EvictorMap } from '@/helpers/offline/eviction'

/**
 * Evictors for the datasets that can actually give space back.
 *
 * Tiles and portraits only, and their relative order is not decided here — `reclaim` walks the
 * declared priority order, so it will not touch portraits until tiles are exhausted.
 *
 * The directory is deliberately absent: it is usually the thing being written when a quota error
 * happens, and evicting it to make room for itself is not progress. The track is absent because it
 * is unrecoverable, and `evictionOrder()` filters it out regardless.
 */
export function browserEvictors(caches: CacheStorageLike | undefined): EvictorMap {
  if (!caches) return {}
  return {
    tiles: tileEvictor(caches, TILE_CACHE_NAME),
    // Portraits reuse the tile evictor's mechanics but not its ordering: they carry no zoom, so
    // every entry parses as unknown and they come off in cache order. Acceptable here — one face is
    // worth much the same as another, unlike one map tile against another.
    portraits: tileEvictor(caches, PORTRAIT_CACHE_NAME),
  }
}
