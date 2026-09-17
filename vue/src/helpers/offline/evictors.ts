// Evictors bound to the real browser caches (task 192).
//
// Kept out of `reporters.ts` so nothing that imports an evictor imports a Pinia store: the contacts
// store needs these to make room for a write, and `reporters.ts` needs the contacts store, which
// would be a cycle through module initialisation for no reason.

import {
  GLIMT_MEDIA_CACHE_NAME,
  GLIMT_THUMB_CACHE_NAME,
  PORTRAIT_CACHE_NAME,
  TILE_CACHE_NAME,
} from '@/config/cache'
import { tileEvictor, type CacheStorageLike, type EvictorMap } from '@/helpers/offline/eviction'

/**
 * Evictors for the datasets that can actually give space back.
 *
 * Their relative order is not decided here — `reclaim` walks the declared priority order in
 * `config/offline.ts`, so it will not touch portraits until tiles are exhausted.
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
    // Glimt: **full-size media first, thumbnails only if that is not enough** (task 315).
    //
    // The order inside this one dataset is the point, and it is why this is a composed evictor
    // rather than one call. Full-size images are ~10× the bytes of a thumbnail and are wanted by one
    // person looking at one photograph, while the thumbnails are what makes a whole grid usable at
    // the finish line. Freeing 6 MB of viewer cache is nearly free; freeing the same from the grid
    // costs two thousand tiles.
    glimt: glimtEvictor(caches),
  }
}

/**
 * Evicts Glimt media, largest-and-least-useful first.
 *
 * Reuses the tile evictor for each cache — neither carries a zoom, so entries come off in cache
 * order, which is a fair proxy for arrival order and therefore roughly oldest-first.
 */
function glimtEvictor(caches: CacheStorageLike) {
  const full = tileEvictor(caches, GLIMT_MEDIA_CACHE_NAME)
  const thumbs = tileEvictor(caches, GLIMT_THUMB_CACHE_NAME)
  return {
    async evict(targetBytes: number) {
      const freed = await full.evict(targetBytes)
      if (freed >= targetBytes) return freed
      // Still short. Take thumbnails too, but only for the remainder — not the original target,
      // which would over-evict the grid by however much the full cache already gave back.
      return freed + (await thumbs.evict(targetBytes - freed))
    },
  }
}
