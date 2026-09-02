import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { PORTRAIT_CACHE_NAME, TILE_CACHE_NAME } from '@/config/cache'
import { OFFLINE_DATASETS } from '@/config/offline'
import { isQuotaExceeded, reclaim, type CacheLike, type CacheStorageLike } from '@/helpers/offline/eviction'
import { browserEvictors } from '@/helpers/offline/evictors'
import { reportCaches } from '@/helpers/offline/reporters'
import { useOfflineStore } from '@/stores/offline.store'

// What happens when the phone is full (PRD 009 §9, task 195).
//
// # Why this file exists separately from eviction.spec.ts
//
// That file tests the eviction *mechanism* in isolation. This one tests the *promises* PRD 009 makes
// to a participant, end to end, against a storage layer that has run out:
//
//   1. unrecoverable data survives;
//   2. map tiles are what gets dropped;
//   3. no cache is emptied wholesale;
//   4. the readiness view can say what happened.
//
// They are worth asserting as a group because each is guarded somewhere different — the declaration,
// the eviction walk, the Workbox config, the store — and it is the *combination* a full phone
// exercises. With no registry enforcing any of it (PRD 009 §4), this is the closest thing to an
// enforcement mechanism the feature has.
//
// The other half of task 195 is a manual protocol: no unit test can tell us what iOS does to a
// home-screen web app it has not seen for a week. See `roadmap/offline-test-protocol.md`.

beforeEach(() => {
  setActivePinia(createPinia())
})

/** A cache that refuses to grow, which is what a full origin looks like from inside. */
function fullCache(urls: string[]): CacheLike & { remaining: () => string[] } {
  const store = new Set(urls)
  return {
    keys: async () => [...store].map((url) => ({ url }) as Request),
    match: async () => ({ headers: { get: () => '32768' } }) as unknown as Response,
    delete: async (request: Request | string) => store.delete(typeof request === 'string' ? request : request.url),
    remaining: () => [...store],
  }
}

function caches(byName: Record<string, CacheLike>): CacheStorageLike & { keys: () => Promise<string[]> } {
  return {
    open: async (name) => byName[name] ?? fullCache([]),
    has: async (name) => name in byName,
    keys: async () => Object.keys(byName),
  }
}

function wmsUrl(zoom: number, n: number): string {
  const span = 40_075_016.686 / 2 ** zoom
  return `https://api.dataforsyningen.dk/dtk_25_DAF?bbox=${n * span},0,${(n + 1) * span},${span}`
}

describe('when the origin is out of space', () => {
  it('keeps the position track, whatever else has to go', async () => {
    const tiles = fullCache([wmsUrl(16, 0), wmsUrl(16, 1), wmsUrl(15, 0)])
    const store = useOfflineStore()

    // Everything an evictor could possibly be offered, including the one it must refuse.
    const result = await store.reclaimSpace(
      {
        ...browserEvictors(caches({ [TILE_CACHE_NAME]: tiles })),
        track: {
          evict: async () => {
            throw new Error('the track was asked to give up data that exists nowhere else')
          },
        },
      },
      64 * 1024,
    )

    expect(result.freedBytes).toBeGreaterThan(0)
    expect(result.evicted).not.toContain('track')
    expect(store.statuses.track.state).not.toBe('evicted')
  })

  it('drops map tiles before portraits or the directory', async () => {
    const tiles = fullCache([wmsUrl(16, 0), wmsUrl(16, 1)])
    const portraits = fullCache(['https://hej/api/contacts/people/a/photo'])
    const store = useOfflineStore()

    await store.reclaimSpace(
      browserEvictors(caches({ [TILE_CACHE_NAME]: tiles, [PORTRAIT_CACHE_NAME]: portraits })),
      32 * 1024,
    )

    expect(tiles.remaining().length).toBeLessThan(2)
    expect(portraits.remaining()).toHaveLength(1)
  })

  it('takes the deepest zoom first, so the map degrades instead of tearing', async () => {
    const tiles = fullCache([wmsUrl(12, 0), wmsUrl(16, 0)])
    const store = useOfflineStore()

    await store.reclaimSpace(browserEvictors(caches({ [TILE_CACHE_NAME]: tiles })), 1)

    // z12–14 is the orientation view someone lost in a forest actually needs; z16 is detail.
    expect(tiles.remaining()).toEqual([wmsUrl(12, 0)])
  })

  it('never empties a cache wholesale to recover', async () => {
    const tiles = fullCache(Array.from({ length: 20 }, (_, i) => wmsUrl(16, i)))
    const store = useOfflineStore()

    // Ask for far more than one tile's worth, but far less than the cache holds.
    await store.reclaimSpace(browserEvictors(caches({ [TILE_CACHE_NAME]: tiles })), 32 * 1024 * 4)

    // A full cache that cannot grow beats an empty one: offline, a discarded tile cannot be
    // re-fetched. This is what refusing Workbox's `purgeOnQuotaError` buys, asserted from the outside.
    expect(tiles.remaining().length).toBeGreaterThan(10)
  })

  it('records what was dropped, so the readiness view can explain it', async () => {
    const tiles = fullCache([wmsUrl(16, 0)])
    const store = useOfflineStore()
    store.report('tiles', { state: 'synced', complete: false, syncedAt: 1_000, bytes: 32_768 })

    await store.reclaimSpace(browserEvictors(caches({ [TILE_CACHE_NAME]: tiles })), 1)

    expect(store.evicted).toContain('tiles')
    // Silent eviction is the failure PRD 009 §9 measures against, and a lost last-synced time would
    // turn "the phone removed some of this" into "you never had it".
    expect(store.statuses.tiles.syncedAt).toBe(1_000)
  })

  it('recognises the error a full origin actually raises', () => {
    // Both spellings, because the legacy name is what older WebKit throws and iOS 16.4 is our floor.
    for (const name of ['QuotaExceededError', 'QUOTA_EXCEEDED_ERR']) {
      const err = new Error('full')
      err.name = name
      expect(isQuotaExceeded(err), name).toBe(true)
    }
  })
})

describe('when the OS has cleared everything', () => {
  // The single most likely real-world failure (PRD 009 §5): iOS clears caches for a web app it has
  // not seen recently, and PRD 005 pushes people to install *early*. The app must explain itself
  // rather than look broken or empty — task 090's white screen is the precedent.
  it('reports every dataset as missing, and nothing as ready', async () => {
    const store = useOfflineStore()
    await reportCaches(caches({}))

    expect(store.ready).toBe(false)
    expect(store.hydrated).toBe(true)

    for (const dataset of ['tiles', 'portraits', 'shell'] as const) {
      // 'empty', not 'unknown': we looked and there is nothing. The difference is the difference
      // between a page that says "mangler" and a page that looks like it is still loading forever.
      expect(store.statuses[dataset].state, dataset).toBe('empty')
    }
  })

  it('names what is missing rather than leaving the user to guess', async () => {
    const store = useOfflineStore()
    await reportCaches(caches({}))

    expect(store.missing.length).toBeGreaterThan(0)
    for (const id of store.missing) {
      expect(OFFLINE_DATASETS.some((d) => d.id === id), id).toBe(true)
    }
  })

  it('survives a storage layer that is not there at all', async () => {
    const store = useOfflineStore()

    // Private mode, an exotic browser, a locked-down device: degrade to live-only, never throw.
    await store.refreshStorage(undefined)
    await reportCaches(undefined)
    const result = await reclaim(browserEvictors(undefined))

    expect(result.freedBytes).toBe(0)
    expect(store.usageBytes).toBeNull()
  })
})
