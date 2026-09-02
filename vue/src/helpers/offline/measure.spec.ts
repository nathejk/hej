import { describe, expect, it } from 'vitest'

import { measureCache, measureShell } from '@/helpers/offline/measure'
import type { CacheLike, CacheStorageLike } from '@/helpers/offline/eviction'

function cacheOf(entries: Record<string, number | null>): CacheLike {
  return {
    keys: async () => Object.keys(entries).map((url) => ({ url }) as Request),
    match: async (request: Request | string) => {
      const length = entries[typeof request === 'string' ? request : request.url]
      if (length === undefined) return undefined
      return {
        headers: { get: () => (length === null ? null : String(length)) },
      } as unknown as Response
    },
    delete: async () => true,
  }
}

function caches(
  byName: Record<string, CacheLike>,
): CacheStorageLike & { keys: () => Promise<string[]> } {
  return {
    open: async (name: string) => byName[name] ?? cacheOf({}),
    has: async (name: string) => name in byName,
    keys: async () => Object.keys(byName),
  }
}

describe('measureCache', () => {
  it('counts entries and sums declared sizes', async () => {
    const api = caches({ tiles: cacheOf({ 'https://x.dk/a': 100, 'https://x.dk/b': 250 }) })
    expect(await measureCache(api, 'tiles')).toEqual({ itemCount: 2, bytes: 350 })
  })

  // Dataforsyningen sends no content-length, so the estimate is the normal path here, not a corner
  // case. It must still produce a believable number rather than zero.
  it('estimates when the response declares no length', async () => {
    const api = caches({ tiles: cacheOf({ 'https://x.dk/?bbox=0,0,611,611': null }) })
    const measured = await measureCache(api, 'tiles')

    expect(measured.itemCount).toBe(1)
    expect(measured.bytes).toBeGreaterThan(0)
  })

  // Before a user has looked at a map, the tile cache genuinely does not exist. That is "nothing
  // stored", not an error, and certainly not a reason to break the profile page.
  it('reports zeros for a cache that does not exist', async () => {
    expect(await measureCache(caches({}), 'tiles')).toEqual({ itemCount: 0, bytes: 0 })
  })

  it('reports zeros when the Cache API is unavailable', async () => {
    expect(await measureCache(undefined, 'tiles')).toEqual({ itemCount: 0, bytes: 0 })
  })

  it('reports zeros rather than throwing when the API fails', async () => {
    const api: CacheStorageLike = {
      open: async () => {
        throw new Error('denied')
      },
    }
    expect(await measureCache(api, 'tiles')).toEqual({ itemCount: 0, bytes: 0 })
  })
})

describe('measureShell', () => {
  // Workbox chooses its own cache names and they change between plugin versions, so this matches by
  // exclusion rather than by a hardcoded name that would silently measure nothing after an upgrade.
  it('sums every cache except our own named ones', async () => {
    const api = caches({
      'workbox-precache-v2-https://hej/': cacheOf({ 'https://hej/index.html': 500 }),
      'workbox-runtime': cacheOf({ 'https://hej/a.js': 100 }),
      'nathejk-map-tiles-v1': cacheOf({ 'https://x.dk/tile': 999_999 }),
    })

    const measured = await measureShell(api, ['nathejk-map-tiles-v1'])

    expect(measured).toEqual({ itemCount: 2, bytes: 600 })
  })

  it('reports zeros when cache names cannot be listed', async () => {
    expect(await measureShell({ open: async () => cacheOf({}) }, [])).toEqual({
      itemCount: 0,
      bytes: 0,
    })
  })
})
