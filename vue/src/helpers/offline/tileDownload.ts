// The bulk tile download (task 087, PRD 002 §11.2).
//
// # What makes this different from the browse-time cache
//
// Tiles are already stored as the map is used, unconditionally and for free — the bytes were being
// fetched anyway. This is the other half: fetching the whole race area *deliberately*, hundreds of
// megabytes, on a connection the app cannot measure. Four properties follow from that, and none of them
// are optional:
//
//   - **resumable** — 5,291 tiles over rural mobile data is minutes. Anything that restarts from zero
//     after a lost signal will never finish, because the conditions that interrupt it recur.
//   - **cancellable, keeping what arrived** — a user who changes their mind must not be punished by
//     losing the part that already worked.
//   - **quota-aware** — a full origin stops the download and says so. It never empties the cache to
//     make room; offline, a discarded tile cannot be re-fetched (PRD 009 §6).
//   - **honest about progress** — a silent multi-minute download is indistinguishable from a hang.
//
// # Why it checks the cache before every fetch
//
// That single check is what makes it resumable, and it costs nothing: the browse-time cache means a
// participant who has looked at the map already holds part of the area, and a second run after an
// interruption holds most of it. There is no separate bookkeeping of "where was I" to get wrong or to
// go stale when the race area changes — the cache *is* the record of progress.

import { isQuotaExceeded, type CacheLike, type CacheStorageLike } from '@/helpers/offline/eviction'

export interface TileDownloadProgress {
  /** Tiles already present or fetched. */
  done: number
  /** Tiles in the whole job. */
  total: number
  /** Tiles fetched over the network in this run, as opposed to already held. */
  fetched: number
  bytes: number
}

export type TileDownloadOutcome = 'complete' | 'cancelled' | 'quota' | 'offline'

export interface TileDownloadResult extends TileDownloadProgress {
  outcome: TileDownloadOutcome
}

export interface TileDownloadOptions {
  /** Tile URLs, in the order they should be fetched. */
  urls: readonly string[]
  cacheName: string
  caches: CacheStorageLike
  fetch: (url: string) => Promise<Response>
  /** Called after each tile, throttled by the caller if it wants. */
  onProgress?: (progress: TileDownloadProgress) => void
  signal?: AbortSignal
  /**
   * How many failed fetches to tolerate before giving up on the run.
   *
   * Not zero, because one 500 from a tile service in the middle of 5,000 requests is normal and
   * abandoning the job for it would be absurd. Not unlimited either: a device that has actually lost
   * signal fails *every* request, and grinding through thousands of them wastes the battery of someone
   * who is going to have to run this again anyway.
   */
  failureLimit?: number
}

export const DEFAULT_FAILURE_LIMIT = 40

/**
 * Fetch and store every tile that is not already stored.
 *
 * Sequential, deliberately. Parallel requests would finish sooner on a good connection and are the wrong
 * trade here: on rural mobile data they compete for one thin pipe, make progress meaningless, and turn a
 * cancellation into "wait for six in-flight requests". The download runs while the user watches a
 * progress bar, so predictability beats throughput.
 */
export async function downloadTiles(options: TileDownloadOptions): Promise<TileDownloadResult> {
  const { urls, caches, fetch, onProgress, signal } = options
  const failureLimit = options.failureLimit ?? DEFAULT_FAILURE_LIMIT

  const progress: TileDownloadProgress = { done: 0, total: urls.length, fetched: 0, bytes: 0 }
  const finish = (outcome: TileDownloadOutcome): TileDownloadResult => ({ ...progress, outcome })

  let cache: CacheLike
  try {
    cache = await caches.open(options.cacheName)
  } catch {
    // No Cache API: nothing to do, and nothing broken either. The map still works online.
    return finish('offline')
  }

  let failures = 0

  for (const url of urls) {
    if (signal?.aborted) return finish('cancelled')

    // The resumability check. Also why a re-run after the race area changes is cheap: tiles the new
    // area shares with the old one are already here, so only the difference is fetched.
    const held = await cache.match(url)
    if (held) {
      progress.done++
      onProgress?.({ ...progress })
      continue
    }

    let response: Response
    try {
      response = await fetch(url)
    } catch {
      // A network failure, which offline means *every* tile.
      if (++failures >= failureLimit) return finish('offline')
      continue
    }

    // Only 200. A tile service answering 500, or answering 200 with an XML ServiceException, must not
    // be stored as though it were map: a cached error is worse than a missing tile, because the missing
    // one is retried and the cached one is believed. (The XML case is why the map's own retry exists.)
    if (!response.ok) {
      if (++failures >= failureLimit) return finish('offline')
      continue
    }

    const size = Number(response.headers.get('content-length') ?? 0)

    try {
      if (!cache.put) return finish('offline')
      await cache.put(url, response)
    } catch (err) {
      if (isQuotaExceeded(err)) {
        // Stop, and report it. Deliberately does NOT evict to continue: the participant asked for the
        // whole area and cannot have it, and quietly discarding tiles they already had to make room for
        // tiles they asked for now would be a strange trade to make on their behalf.
        return finish('quota')
      }
      // Anything else — a storage error, a locked cache — is treated like a failed fetch.
      if (++failures >= failureLimit) return finish('offline')
      continue
    }

    progress.done++
    progress.fetched++
    if (Number.isFinite(size)) progress.bytes += size
    onProgress?.({ ...progress })
  }

  return finish(signal?.aborted ? 'cancelled' : 'complete')
}
