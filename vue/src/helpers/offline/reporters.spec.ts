import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { PORTRAIT_CACHE_NAME, TILE_CACHE_NAME } from '@/config/cache'
import type { CacheLike, CacheStorageLike } from '@/helpers/offline/eviction'
import { browserEvictors } from '@/helpers/offline/evictors'
import { purgeSensitiveData, registerOfflineDatasets, reportCaches } from '@/helpers/offline/reporters'
import { useOfflineStore } from '@/stores/offline.store'

beforeEach(() => {
  setActivePinia(createPinia())
})

function cacheOf(urls: string[], bytes = 1000): CacheLike {
  const store = new Set(urls)
  return {
    keys: async () => [...store].map((url) => ({ url }) as Request),
    match: async () => ({ headers: { get: () => String(bytes) } }) as unknown as Response,
    delete: async (request: Request | string) => store.delete(typeof request === 'string' ? request : request.url),
  }
}

function caches(
  byName: Record<string, CacheLike>,
): CacheStorageLike & { keys: () => Promise<string[]> } {
  return {
    open: async (name) => byName[name] ?? cacheOf([]),
    has: async (name) => name in byName,
    keys: async () => Object.keys(byName),
  }
}

describe('reportCaches', () => {
  it('reports tiles, portraits and the shell separately', async () => {
    const api = caches({
      [TILE_CACHE_NAME]: cacheOf(['https://x.dk/t1', 'https://x.dk/t2']),
      [PORTRAIT_CACHE_NAME]: cacheOf(['https://hej/api/contacts/people/a/photo']),
      'workbox-precache-v2': cacheOf(['https://hej/index.html']),
    })

    const store = useOfflineStore()
    await reportCaches(api)

    expect(store.statuses.tiles.itemCount).toBe(2)
    expect(store.statuses.portraits.itemCount).toBe(1)
    expect(store.statuses.shell.itemCount).toBe(1)
    // The shell's bytes must not include the tile cache — the whole reason for a per-dataset cache
    // name is that one undifferentiated bucket cannot be attributed, evicted or purged.
    expect(store.statuses.shell.bytes).toBe(1000)
  })

  // Tiles arrive as the map is browsed (task 087's cheap half), so a non-empty cache means "some of
  // the map", never "the map". Reporting complete here would put "Klar" beside a map with holes.
  it('never reports tiles as complete, however many are cached', async () => {
    const api = caches({ [TILE_CACHE_NAME]: cacheOf(Array.from({ length: 50 }, (_, i) => `u${i}`)) })

    const store = useOfflineStore()
    await reportCaches(api)

    expect(store.statuses.tiles.state).toBe('synced')
    expect(store.statuses.tiles.complete).toBe(false)
    expect(store.ready).toBe(false)
  })

  // The shell is the one genuinely all-or-nothing dataset: if the precache had not installed, the
  // app the user is reading this in would not be running.
  it('reports the shell as complete when anything is precached', async () => {
    const api = caches({ 'workbox-precache-v2': cacheOf(['https://hej/index.html']) })

    const store = useOfflineStore()
    await reportCaches(api)

    expect(store.statuses.shell.complete).toBe(true)
  })

  it('reports empty rather than unknown when a cache is absent', async () => {
    const store = useOfflineStore()
    await reportCaches(caches({}))

    expect(store.statuses.tiles.state).toBe('empty')
    expect(store.statuses.portraits.state).toBe('empty')
  })

  it('survives the Cache API being unavailable', async () => {
    const store = useOfflineStore()
    await reportCaches(undefined)

    expect(store.statuses.tiles.state).toBe('empty')
  })
})

describe('purgeSensitiveData', () => {
  // Both halves, or neither. A directory of names with no faces and a set of faces with no names are
  // both worse than neither, and a half-done purge is the kind of thing that reads as done.
  it('deletes the portrait bytes as well as the index', async () => {
    const portraits = cacheOf(['https://hej/api/contacts/people/a/photo'])
    const api = caches({ [PORTRAIT_CACHE_NAME]: portraits })

    const store = useOfflineStore()
    store.report('directory', { state: 'synced', complete: true, syncedAt: 5 })
    store.report('portraits', { state: 'synced', itemCount: 1 })

    await purgeSensitiveData(api)

    expect(await portraits.keys()).toHaveLength(0)
    expect(store.statuses.directory.state).toBe('empty')
    expect(store.statuses.portraits.state).toBe('empty')
    // The last-synced time survives a purge: "you had this until the event ended" is a different and
    // more honest thing to show than "you never had it".
    expect(store.statuses.directory.syncedAt).toBe(5)
  })

  it('still clears the index when the Cache API is unavailable', async () => {
    const store = useOfflineStore()
    store.report('directory', { state: 'synced', complete: true })

    await purgeSensitiveData(undefined)

    expect(store.statuses.directory.state).toBe('empty')
  })
})

describe('tile download handlers', () => {
  it('offers sync, cancel and clear for tiles once the Cache API exists', async () => {
    const api = caches({ [TILE_CACHE_NAME]: cacheOf([]) })
    const store = useOfflineStore()

    await registerOfflineDatasets(api)

    expect(Boolean(store.handlers.tiles?.sync)).toBe(true)
    expect(Boolean(store.handlers.tiles?.cancel)).toBe(true)
    expect(Boolean(store.handlers.tiles?.clear)).toBe(true)
  })

  // A progress bar with no way out is a trap: this download runs for minutes on rural mobile data.
  it('reports tiles as cancellable only while a download is running', async () => {
    const api = caches({ [TILE_CACHE_NAME]: cacheOf([]) })
    const store = useOfflineStore()
    await registerOfflineDatasets(api)

    expect(store.cancellable).toEqual([])
    store.report('tiles', { state: 'syncing' })
    expect(store.cancellable).toEqual(['tiles'])
  })

  it('turns a countable progress report into a percentage', () => {
    const store = useOfflineStore()
    expect(store.syncPercent).toBeNull()

    store.report('tiles', { state: 'syncing', progress: { done: 1_323, total: 5_291 } })
    expect(store.syncPercent).toBe(25)
  })

  // Progress belongs to a running job. Left behind, a finished download would show for ever as though it
  // were still at 4,912 of 5,291.
  it('clears progress when a sync ends', async () => {
    const store = useOfflineStore()
    store.registerHandlers('tiles', {
      sync: async () => {
        store.report('tiles', { progress: { done: 5, total: 10 } })
      },
    })

    await store.sync('tiles')

    expect(store.statuses.tiles.progress).toBeNull()
  })
})

describe('the index/binary separation', () => {
  // The point of keeping the directory's text in one dataset and its faces in another: losing the
  // faces must leave names, groups and phone numbers working. PRD 007 depends on this — search at
  // 03:00 has to work on a phone whose portrait cache the OS took.
  it('leaves the directory intact when portraits are evicted', async () => {
    const store = useOfflineStore()
    store.report('directory', { state: 'synced', complete: true, itemCount: 151, bytes: 90_000 })
    store.report('portraits', { state: 'synced', complete: false, itemCount: 151 })

    store.markEvicted('portraits')

    expect(store.statuses.directory.state).toBe('synced')
    expect(store.statuses.directory.complete).toBe(true)
    expect(store.statuses.directory.itemCount).toBe(151)
  })

  it('measures them as separate caches, so one cannot hide the other', async () => {
    const api = caches({
      [PORTRAIT_CACHE_NAME]: cacheOf(['https://hej/api/contacts/people/a/photo']),
      [TILE_CACHE_NAME]: cacheOf(['https://x.dk/t']),
    })

    const store = useOfflineStore()
    await reportCaches(api)

    expect(store.statuses.portraits.bytes).toBe(1000)
    expect(store.statuses.tiles.bytes).toBe(1000)
  })
})

describe('browserEvictors', () => {
  it('offers tiles and portraits, and nothing else', () => {
    const keys = Object.keys(browserEvictors(caches({})))
    expect(keys.sort()).toEqual(['portraits', 'tiles'])
  })

  // The directory is usually the thing being written when a quota error happens; evicting it to make
  // room for itself is not progress. The track is unrecoverable and must never be offered anywhere.
  it('never offers the directory or the track', () => {
    const evictors = browserEvictors(caches({})) as Record<string, unknown>
    expect(evictors.directory).toBeUndefined()
    expect(evictors.track).toBeUndefined()
  })

  it('offers nothing when the Cache API is unavailable', () => {
    expect(browserEvictors(undefined)).toEqual({})
  })

  it('actually deletes from the named cache it was given', async () => {
    const tiles = cacheOf(['https://x.dk/?bbox=0,0,611,611'])
    const evictors = browserEvictors(caches({ [TILE_CACHE_NAME]: tiles }))

    const freed = await evictors.tiles!.evict(1)

    expect(freed).toBeGreaterThan(0)
    expect(await tiles.keys()).toHaveLength(0)
  })
})

describe('when there is no race area to download', () => {
  // The bug reported from a phone on 2026-09-02: tapping "Hent nu" on Kortbilleder flickered to "Hentes…"
  // for half a second, went back to "Klar", and the size did not change. `/api/race-area` answers 404 when
  // no checkpoint has coordinates yet, and the handler swallowed it — the same silent-failure shape as task
  // 197, in a different feature. Half a second is the round trip.
  it('reports a cause rather than flickering silently', async () => {
    const store = useOfflineStore()
    store.report('tiles', { state: 'synced', itemCount: 252, bytes: 26_500_000, complete: false })

    // Stand in for the handler's own outcome, which is what the reporter sets.
    store.report('tiles', { problem: 'no-area' })

    expect(store.statuses.tiles.problem).toBe('no-area')
    // And nothing about what is already cached is disturbed: 252 tiles from browsing are still there.
    expect(store.statuses.tiles.itemCount).toBe(252)
  })

  // A handler that throws used to be swallowed by `sync()` entirely, producing exactly the same flicker.
  // Whatever the cause turns out to be, the app must not be the thing that hides it.
  it('surfaces a handler that throws', async () => {
    const store = useOfflineStore()
    store.report('tiles', { state: 'synced', complete: false })
    store.registerHandlers('tiles', {
      sync: async () => {
        throw new Error('leaflet exploded')
      },
    })

    await store.sync('tiles')

    expect(store.statuses.tiles.problem).toBe('error')
    // The previous state is restored rather than invented: a failed download does not empty the cache.
    expect(store.statuses.tiles.state).toBe('synced')
    expect(store.statuses.tiles.progress).toBeNull()
  })
})
