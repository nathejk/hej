import { describe, expect, it } from 'vitest'

import {
  EVICTION_TARGET_BYTES,
  estimateResponseBytes,
  isQuotaExceeded,
  reclaim,
  tileEvictor,
  wholeValueEvictor,
  type CacheLike,
  type CacheStorageLike,
  type DatasetEvictor,
} from '@/helpers/offline/eviction'

// A Cache API stand-in. Only what the evictor touches: keys, match, delete.
function fakeCache(urls: string[], bytes = 32 * 1024): CacheLike & { remaining: () => string[] } {
  const store = new Map(urls.map((url) => [url, bytes]))
  return {
    keys: async () => [...store.keys()].map((url) => ({ url }) as Request),
    match: async (request: Request) =>
      store.has(request.url)
        ? ({ headers: { get: () => String(store.get(request.url)) } } as unknown as Response)
        : undefined,
    delete: async (request: Request) => store.delete(request.url),
    remaining: () => [...store.keys()],
  }
}

function fakeCaches(cache: CacheLike): CacheStorageLike {
  return { open: async () => cache }
}

function wmsUrl(zoom: number, n: number): string {
  const span = 40_075_016.686 / 2 ** zoom
  return `https://api.dataforsyningen.dk/dtk_25_DAF?bbox=${n * span},0,${(n + 1) * span},${span}`
}

describe('isQuotaExceeded', () => {
  it('recognises the modern and legacy names', () => {
    const modern = new Error('full')
    modern.name = 'QuotaExceededError'
    const legacy = new Error('full')
    legacy.name = 'QUOTA_EXCEEDED_ERR'

    expect(isQuotaExceeded(modern)).toBe(true)
    expect(isQuotaExceeded(legacy)).toBe(true)
  })

  it('does not swallow unrelated failures', () => {
    expect(isQuotaExceeded(new Error('network'))).toBe(false)
    expect(isQuotaExceeded('QuotaExceededError')).toBe(false)
    expect(isQuotaExceeded(undefined)).toBe(false)
  })
})

describe('reclaim', () => {
  // The invariant that matters most in the whole feature: the track is not reachable from here,
  // even by a caller that provides an evictor for it and keeps asking.
  it('never evicts an unrecoverable dataset, even when one is offered', async () => {
    let trackAsked = false
    const evictors = {
      track: {
        evict: async () => {
          trackAsked = true
          return 1_000_000
        },
      } satisfies DatasetEvictor,
    }

    const result = await reclaim(evictors, 500_000)

    expect(trackAsked).toBe(false)
    expect(result.freedBytes).toBe(0)
    expect(result.evicted).toEqual([])
  })

  it('takes from tiles before anything else', async () => {
    const asked: string[] = []
    const spy = (id: string, freed: number): DatasetEvictor => ({
      evict: async () => {
        asked.push(id)
        return freed
      },
    })

    const result = await reclaim(
      { tiles: spy('tiles', 100), portraits: spy('portraits', 100), directory: spy('directory', 100) },
      100,
    )

    expect(asked).toEqual(['tiles'])
    expect(result.evicted).toEqual(['tiles'])
  })

  it('moves up the order until the target is met', async () => {
    const asked: string[] = []
    const spy = (id: string, freed: number): DatasetEvictor => ({
      evict: async () => {
        asked.push(id)
        return freed
      },
    })

    const result = await reclaim(
      { tiles: spy('tiles', 40), portraits: spy('portraits', 40), directory: spy('directory', 40) },
      100,
    )

    expect(asked).toEqual(['tiles', 'portraits', 'directory'])
    expect(result.freedBytes).toBe(120)
  })

  // One broken adapter must not strand the ones below it — a failure here is most likely storage
  // being unavailable, in which case there was nothing to reclaim there anyway.
  it('carries on when an evictor throws', async () => {
    const result = await reclaim(
      {
        tiles: {
          evict: async () => {
            throw new Error('cache unavailable')
          },
        },
        portraits: { evict: async () => 50 },
      },
      50,
    )

    expect(result.evicted).toEqual(['portraits'])
    expect(result.freedBytes).toBe(50)
  })

  it('does not record a dataset that freed nothing', async () => {
    const result = await reclaim({ tiles: { evict: async () => 0 } }, 100)
    expect(result.evicted).toEqual([])
  })

  it('frees a generous default rather than exactly one failed write', () => {
    // Freeing exactly enough would mean an eviction pass on every subsequent tile — hundreds of
    // full cache scans while a participant pans a map.
    expect(EVICTION_TARGET_BYTES).toBeGreaterThan(1024 * 1024)
  })
})

describe('tileEvictor', () => {
  it('deletes the highest zoom first', async () => {
    const cache = fakeCache([wmsUrl(12, 0), wmsUrl(16, 0), wmsUrl(14, 0)])
    // One tile's worth, so exactly one deletion happens and the choice is visible.
    await tileEvictor(fakeCaches(cache)).evict(1)

    const zooms = cache.remaining()
    expect(zooms).toHaveLength(2)
    expect(zooms).not.toContain(wmsUrl(16, 0))
  })

  it('works down through the zooms when more space is needed', async () => {
    const cache = fakeCache([wmsUrl(16, 0), wmsUrl(16, 1), wmsUrl(15, 0), wmsUrl(12, 0)])
    await tileEvictor(fakeCaches(cache)).evict(32 * 1024 * 3)

    expect(cache.remaining()).toEqual([wmsUrl(12, 0)])
  })

  it('stops as soon as the target is met', async () => {
    const cache = fakeCache([wmsUrl(16, 0), wmsUrl(16, 1), wmsUrl(16, 2)], 100)
    const freed = await tileEvictor(fakeCaches(cache)).evict(150)

    expect(freed).toBeGreaterThanOrEqual(150)
    expect(cache.remaining()).toHaveLength(1)
  })

  // An unrecognised entry is more likely to be something this code does not understand than a
  // tile we meant to discard. Deleting those first would make a URL-shape change look like
  // working eviction while quietly emptying the cache.
  it('evicts entries of unknown zoom last', async () => {
    const cache = fakeCache(['https://api.dataforsyningen.dk/dtk_25_DAF?service=WMS', wmsUrl(16, 0)])
    await tileEvictor(fakeCaches(cache)).evict(1)

    expect(cache.remaining()).toEqual(['https://api.dataforsyningen.dk/dtk_25_DAF?service=WMS'])
  })

  // A stray zoom outside the declared order must still be reachable, or it would be undeletable.
  it('evicts an undeclared deep zoom before the declared ones', async () => {
    const cache = fakeCache([wmsUrl(16, 0), wmsUrl(17, 0)])
    await tileEvictor(fakeCaches(cache)).evict(1)

    expect(cache.remaining()).toEqual([wmsUrl(16, 0)])
  })

  it('returns zero from an empty cache rather than failing', async () => {
    const cache = fakeCache([])
    expect(await tileEvictor(fakeCaches(cache)).evict(1000)).toBe(0)
  })
})

describe('estimateResponseBytes', () => {
  it('prefers content-length when the service sends it', () => {
    const response = { headers: { get: () => '4242' } } as unknown as Response
    expect(estimateResponseBytes(response, 16)).toBe(4242)
  })

  // Dataforsyningen sends no cache headers at all, so the fallback is the normal path, not a
  // corner case. Numbers measured inside the race area in task 087.
  it('falls back to measured per-zoom sizes, largest at the shallowest zoom', () => {
    expect(estimateResponseBytes(undefined, 12)).toBeGreaterThan(estimateResponseBytes(undefined, 16))
  })

  it('has a fallback for an unknown zoom', () => {
    expect(estimateResponseBytes(undefined, null)).toBeGreaterThan(0)
  })
})

describe('wholeValueEvictor', () => {
  it('reports the size it actually freed', async () => {
    let cleared = false
    const evictor = wholeValueEvictor(
      () => 'x'.repeat(1000),
      () => {
        cleared = true
      },
    )

    // UTF-16: two bytes per code unit.
    expect(await evictor.evict(1)).toBe(2000)
    expect(cleared).toBe(true)
  })

  it('frees nothing, and clears nothing, when there is no value', async () => {
    let cleared = false
    const evictor = wholeValueEvictor(
      () => null,
      () => {
        cleared = true
      },
    )

    expect(await evictor.evict(1)).toBe(0)
    expect(cleared).toBe(false)
  })
})
