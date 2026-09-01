// Wiring the four existing caches into PRD 009's shared budget and readiness surface (task 192).
//
// # This is the task that decides whether PRD 009 took effect
//
// The declaration (`config/offline.ts`), the store (`offline.store`), the eviction policy and the
// readiness view all shipped inert. Nothing observed any of it, because every cache in this app was
// written before the policy existed: tiles in a Workbox route, the directory in `localStorage`, the
// track in IndexedDB, the shell in the precache. This module is what connects them — and nothing
// more. No cache is rewritten, no storage is moved.
//
// # Why reporting is pulled, not pushed
//
// Three of the four datasets cannot tell us when they change: the tile cache is written by the
// service worker, the precache by Workbox at install time, and neither notifies the page. So the
// app measures them on demand — at startup and when the readiness view is opened — rather than
// pretending to a live count. The directory, which *is* written by app code, still reports on write
// because it can.

import { PORTRAIT_CACHE_NAME, TILE_CACHE_NAME } from '@/config/cache'
import type { CacheStorageLike } from '@/helpers/offline/eviction'
import { measureCache, measureShell } from '@/helpers/offline/measure'
import { countPoints, countPending } from '@/helpers/trackDb'
import { useContactsStore } from '@/stores/contacts.store'
import { useOfflineStore } from '@/stores/offline.store'
import { useSessionStore } from '@/stores/session.store'

/** Our own named caches, so shell measurement can exclude them. */
export const OWN_CACHE_NAMES = [TILE_CACHE_NAME, PORTRAIT_CACHE_NAME] as const

type CachesApi = CacheStorageLike & { keys?: () => Promise<string[]> }

/** Measure the binary caches and report them. Safe to call repeatedly. */
export async function reportCaches(caches: CachesApi | undefined) {
  const offline = useOfflineStore()

  const [tiles, portraits, shell] = await Promise.all([
    measureCache(caches, TILE_CACHE_NAME),
    measureCache(caches, PORTRAIT_CACHE_NAME),
    measureShell(caches, OWN_CACHE_NAMES),
  ])

  offline.report('tiles', {
    // Never `complete`. Tiles are cached as they are browsed today (task 087's cheap half), so a
    // non-empty cache means "some of the map", never "the map" — and the bulk download that could
    // honestly claim completeness is still task 087's second half. Reporting anything else here
    // would put "Klar" next to a map with holes in it.
    state: tiles.itemCount > 0 ? 'synced' : 'empty',
    complete: false,
    itemCount: tiles.itemCount,
    bytes: tiles.bytes,
  })

  offline.report('portraits', {
    state: portraits.itemCount > 0 ? 'synced' : 'empty',
    // Same reasoning: portraits arrive as rows are drawn, so we know we have *some*.
    complete: false,
    itemCount: portraits.itemCount,
    bytes: portraits.bytes,
  })

  offline.report('shell', {
    state: shell.itemCount > 0 ? 'synced' : 'empty',
    // The shell is the one dataset that genuinely is all-or-nothing: Workbox's precache either
    // installed or it did not, and if it did not, the app the user is reading this in would not
    // be running.
    complete: shell.itemCount > 0,
    itemCount: shell.itemCount,
    bytes: shell.bytes,
  })
}

/** Measure the position track from its own store and report it. */
export async function reportTrack() {
  const offline = useOfflineStore()
  const userId = useSessionStore().user?.userId

  try {
    const [points, pending] = await Promise.all([
      countPoints(),
      // Pending is per-user; nobody signed in means nothing of ours is waiting.
      userId ? countPending(userId) : Promise.resolve(0),
    ])

    offline.report('track', {
      state: points > 0 ? 'synced' : 'empty',
      itemCount: points,
      // ~135 bytes per point as stored (task 082's measurement: ~195 KB for 1,440 points).
      bytes: points * 135,
      // "Complete" for the track means nothing is waiting to be uploaded. That is the only
      // reading with a meaning: an unshipped point is the one piece of data in this app that
      // exists nowhere else, so a backlog is exactly what the user should be told about.
      complete: pending === 0,
    })
  } catch {
    // IndexedDB unavailable (private mode). Leaving the row as 'unknown' is honest.
  }
}

/** Report the contacts directory from what the store already knows. */
export function reportDirectory() {
  const offline = useOfflineStore()
  const contacts = useContactsStore()

  if (contacts.forbidden) {
    // This role has no contacts pane at all, so an empty row would be a lie by omission — there is
    // nothing missing. Reported as complete-and-empty rather than 'Mangler'.
    offline.report('directory', { state: 'synced', complete: true, itemCount: 0, bytes: 0 })
    return
  }

  offline.report('directory', {
    state: contacts.entries.length > 0 ? 'synced' : 'empty',
    complete: contacts.entries.length > 0,
    itemCount: contacts.byId.size,
    // The stored JSON is what occupies the quota, not the in-memory objects. UTF-16 in
    // localStorage, hence the doubling.
    bytes: contacts.entries.length ? JSON.stringify(contacts.entries).length * 2 : 0,
    expiresAt: contacts.expiresAt || null,
  })
}

/**
 * Delete the sensitive datasets: the directory index and the portrait bytes, together.
 *
 * Together on purpose. A directory of names with no faces and a set of faces with no names are both
 * worse than neither, and half a purge is the kind of thing that reads as done and is not. The
 * client-side half of task 193; the server keeps its own schedule (`portraitpurge.go`).
 *
 * Called when the index turns out to be past its server-issued deadline — which, on a device that
 * has not opened the app since the event, is the only moment anything of ours will ever run again.
 */
export async function purgeSensitiveData(caches: CachesApi | undefined) {
  const contacts = useContactsStore()
  const offline = useOfflineStore()

  contacts.clearLocalCopy()
  offline.markCleared('directory')

  if (caches) {
    try {
      const cache = await caches.open(PORTRAIT_CACHE_NAME)
      for (const key of await cache.keys()) await cache.delete(key)
    } catch {
      // Nothing to do but carry on: the cache's own 14-day `maxAgeSeconds` is the backstop, which is
      // why the two numbers were deliberately set to match.
    }
  }
  offline.markCleared('portraits')
}

/**
 * Connect every existing cache to the readiness surface, and register what the user may ask of it.
 *
 * Called once from `App.vue`. Handlers are registered for the two datasets that have something
 * honest to offer:
 *
 * - **directory** — refetch, and clear. Both already exist as store actions.
 * - **tiles** — clear only. There is no bulk download yet (task 087), and a "hent nu" that only
 *   fetched whatever the map last showed would be a button that appears to work and does almost
 *   nothing.
 *
 * The shell, portraits and track get none: the shell cannot usefully be re-fetched by a user,
 * portraits arrive with the rows that need them, and the track has nothing to fetch and must never
 * be clearable (PRD 009 §6).
 */
export async function registerOfflineDatasets(caches: CachesApi | undefined) {
  const offline = useOfflineStore()
  const contacts = useContactsStore()

  // Enforce the retention deadline before anything else reads the copy.
  //
  // `hydrate` drops an expired payload from storage and sets `expired`; the flag exists because the
  // *other* half of the purge is the portrait bytes, which live in a cache the contacts store knows
  // nothing about. Running the check on every launch is the point: a device that never reopens the
  // app after the event is exactly the one no server-side purge can reach.
  contacts.hydrate()
  if (contacts.expired) await purgeSensitiveData(caches)

  offline.registerHandlers('directory', {
    sync: async () => {
      await contacts.fetch()
      reportDirectory()
    },
    clear: () => {
      contacts.clearLocalCopy()
      reportDirectory()
    },
  })

  if (caches) {
    offline.registerHandlers('tiles', {
      clear: async () => {
        const cache = await caches.open(TILE_CACHE_NAME)
        for (const key of await cache.keys()) await cache.delete(key)
        await reportCaches(caches)
      },
    })
  }

  await Promise.all([reportCaches(caches), reportTrack()])
  reportDirectory()
}
