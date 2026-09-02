import { describe, expect, it } from 'vitest'

import { downloadTiles, type TileDownloadOptions } from '@/helpers/offline/tileDownload'
import type { CacheLike, CacheStorageLike } from '@/helpers/offline/eviction'

function urlFor(n: number): string {
  return `https://api.dataforsyningen.dk/dtk_25_DAF?bbox=${n},0,${n + 1},1`
}

function fakeCache(held: string[] = []): CacheLike & { stored: () => string[] } {
  const store = new Set(held)
  return {
    keys: async () => [...store].map((url) => ({ url }) as Request),
    match: async (request) => {
      const url = typeof request === 'string' ? request : request.url
      return store.has(url) ? ({} as Response) : undefined
    },
    delete: async () => true,
    put: async (request) => {
      store.add(typeof request === 'string' ? request : request.url)
    },
    stored: () => [...store],
  }
}

function caches(cache: CacheLike): CacheStorageLike {
  return { open: async () => cache }
}

function ok(bytes = 32_768): Response {
  return { ok: true, headers: { get: () => String(bytes) } } as unknown as Response
}

function options(over: Partial<TileDownloadOptions> = {}): TileDownloadOptions {
  const cache = fakeCache()
  return {
    urls: [urlFor(1), urlFor(2), urlFor(3)],
    cacheName: 'tiles',
    caches: caches(cache),
    fetch: async () => ok(),
    ...over,
  }
}

describe('downloadTiles', () => {
  it('fetches and stores every tile', async () => {
    const cache = fakeCache()
    const result = await downloadTiles(options({ caches: caches(cache) }))

    expect(result.outcome).toBe('complete')
    expect(result.done).toBe(3)
    expect(result.fetched).toBe(3)
    expect(cache.stored()).toHaveLength(3)
    expect(result.bytes).toBe(3 * 32_768)
  })

  // Resumability, and the whole reason there is no separate progress bookkeeping: the cache *is* the
  // record of what has been done. 5,000 tiles over rural mobile data is minutes, and anything that
  // restarts from zero after a lost signal will never finish, because the conditions recur.
  it('skips tiles it already holds, so an interrupted run resumes', async () => {
    const cache = fakeCache([urlFor(1), urlFor(2)])
    let fetched = 0

    const result = await downloadTiles(
      options({
        caches: caches(cache),
        fetch: async () => {
          fetched++
          return ok()
        },
      }),
    )

    expect(fetched).toBe(1)
    expect(result.done).toBe(3)
    expect(result.fetched).toBe(1)
    expect(result.outcome).toBe('complete')
  })

  it('reports progress as it goes, so a long download is not a hang', async () => {
    const seen: number[] = []
    await downloadTiles(options({ onProgress: (p) => seen.push(p.done) }))

    expect(seen).toEqual([1, 2, 3])
  })

  // A user who changes their mind must not be punished by losing the part that already worked.
  it('stops when cancelled and keeps what it already stored', async () => {
    const cache = fakeCache()
    const controller = new AbortController()
    let fetched = 0

    const result = await downloadTiles(
      options({
        urls: [urlFor(1), urlFor(2), urlFor(3), urlFor(4)],
        caches: caches(cache),
        fetch: async () => {
          if (++fetched === 2) controller.abort()
          return ok()
        },
        signal: controller.signal,
      }),
    )

    expect(result.outcome).toBe('cancelled')
    expect(cache.stored()).toHaveLength(2)
    expect(result.done).toBe(2)
  })

  it('does not start at all when already cancelled', async () => {
    const controller = new AbortController()
    controller.abort()
    let fetched = 0

    const result = await downloadTiles(
      options({
        fetch: async () => {
          fetched++
          return ok()
        },
        signal: controller.signal,
      }),
    )

    expect(fetched).toBe(0)
    expect(result.outcome).toBe('cancelled')
  })

  // Stops and says so. Deliberately does not evict to continue: quietly discarding tiles the
  // participant already had, to make room for tiles they asked for now, is a strange trade to make on
  // their behalf — and offline a discarded tile cannot be re-fetched.
  it('stops on a full origin without emptying anything', async () => {
    const cache = fakeCache()
    const full = { ...cache, put: async () => {
      const err = new Error('full')
      err.name = 'QuotaExceededError'
      throw err
    } }

    const result = await downloadTiles(options({ caches: caches(full) }))

    expect(result.outcome).toBe('quota')
    expect(cache.stored()).toEqual([])
  })

  // One 500 in the middle of 5,000 requests is normal; abandoning the job for it would be absurd.
  it('tolerates the occasional failure', async () => {
    let n = 0
    const cache = fakeCache()

    const result = await downloadTiles(
      options({
        caches: caches(cache),
        fetch: async () => (++n === 2 ? ({ ok: false, headers: { get: () => null } } as unknown as Response) : ok()),
      }),
    )

    expect(result.outcome).toBe('complete')
    expect(result.fetched).toBe(2)
    expect(cache.stored()).toHaveLength(2)
  })

  // But a device that has actually lost signal fails *every* request, and grinding through thousands of
  // them wastes the battery of someone who will have to run this again anyway.
  it('gives up when everything fails', async () => {
    const result = await downloadTiles(
      options({
        urls: Array.from({ length: 100 }, (_, i) => urlFor(i)),
        fetch: async () => {
          throw new Error('offline')
        },
        failureLimit: 5,
      }),
    )

    expect(result.outcome).toBe('offline')
    expect(result.fetched).toBe(0)
  })

  // A cached error is worse than a missing tile: the missing one is retried, the cached one is believed.
  it('never stores a non-200 response', async () => {
    const cache = fakeCache()

    await downloadTiles(
      options({
        caches: caches(cache),
        fetch: async () => ({ ok: false, headers: { get: () => null } } as unknown as Response),
        failureLimit: 99,
      }),
    )

    expect(cache.stored()).toEqual([])
  })

  it('degrades to doing nothing when there is no Cache API', async () => {
    const result = await downloadTiles(
      options({
        caches: {
          open: async () => {
            throw new Error('no caches')
          },
        },
      }),
    )

    expect(result.outcome).toBe('offline')
    expect(result.done).toBe(0)
  })
})
