import { useOnboardingStore } from '@/stores/onboarding.store'
import { useSessionStore } from '@/stores/session.store'
import { useOfflineStore } from '@/stores/offline.store'
import { OFFLINE_DATASETS } from '@/config/offline'

import { clearDevOverrides } from '@/dev/devDevice'

// The dev panel's resets (PRD 014, task 209).
//
// This is the task that actually buys the speed PRD 014 was asked for. Simulating a phone gets
// you *into* onboarding once; testing step 3 of it twenty times needs the state gone twenty
// times, and that state is spread across localStorage, IndexedDB, the cookie jar and the
// service worker's caches. The target is a full re-run in under 10 seconds with no DevTools.
//
// # These are callers, not new logic
//
// Every reset goes through the owner's own action:
//
// - `onboardingStore.reset()` already exists and already clears the persisted completion flag.
//   Its comment says it is used by the dev/QA override *and by sign-out*, so this button
//   exercises the same code the product does — if it stops working, sign-out is broken too.
// - `sessionStore.logout()` posts /api/auth/logout and drops the remembered identity even when
//   the request fails, so it is preferable to clearing storage by hand.
// - Dataset clearing goes through `offlineStore.clear(id)`, which dispatches to the handler the
//   owning feature registered. Reaching past those handlers into IndexedDB would be the same
//   mistake that store's header forbids ("anything here that starts fetching or evicting is a
//   sign the registry has been reinvented") — and it would leave the dataset's *status* wrong,
//   because the store's bookkeeping would not run. The readiness view would then report
//   something that is not true, which is precisely the class of bug PRD 009 exists to prevent.

export type ResetTarget = 'onboarding' | 'session' | 'caches' | 'everything'

// The runtime config's localStorage mirror (see @/config/runtime). Cleared with the caches
// because it is a cache: values the BFF served, remembered so an offline start still works.
const RUNTIME_MIRROR_KEYS = [
  'hej.dataforsyningen-token',
  'hej.show-build-id',
  'hej.show-layout-debug',
  'hej.install-gate',
  'hej.contacts-poll-seconds',
]

// The track's IndexedDB database, from @/helpers/trackDb.
const TRACK_DB = 'hej-track'

function forget(keys: string[]) {
  for (const key of keys) {
    try {
      localStorage.removeItem(key)
    } catch {
      // Dev-only nuisance.
    }
  }
}

async function clearDatasets(): Promise<string[]> {
  const offline = useOfflineStore()
  const cleared: string[] = []
  for (const dataset of OFFLINE_DATASETS) {
    // `clear()` refuses anything marked unrecoverable, and the position track is the only such
    // dataset. That refusal is correct in the product — the local copy may be the sole record
    // of where a team was — so it is not bypassed here; the track is handled separately below,
    // where the reasoning is that a *developer's* fake track is not evidence of anything.
    if (dataset.unrecoverable) continue
    try {
      await offline.clear(dataset.id)
      cleared.push(dataset.id)
    } catch {
      // One failing dataset must not stop the others — the same posture as the evictor loop
      // in @/helpers/offline/eviction.
    }
  }
  return cleared
}

// Deleting the track database directly, which nothing in the product does or should.
//
// `offlineStore.clear('track')` refuses by design, and rightly: for a participant that data is
// irreplaceable. In a dev environment it is a fake walk produced by task 213's playback, and
// leaving it behind makes every subsequent track test start from someone else's route. So the
// refusal is respected above and stepped around *here*, in a dev-only module, deliberately and
// in one place rather than by weakening the store.
async function deleteTrackDb(): Promise<boolean> {
  if (typeof indexedDB === 'undefined') return false
  return new Promise((resolve) => {
    const req = indexedDB.deleteDatabase(TRACK_DB)
    req.onsuccess = () => resolve(true)
    req.onerror = () => resolve(false)
    // Fires when another tab still holds the database open. Resolving rather than hanging: the
    // developer gets a report that says so, instead of a button that never finishes.
    req.onblocked = () => resolve(false)
  })
}

async function clearCacheStorage(): Promise<number> {
  if (typeof caches === 'undefined') return 0
  const names = await caches.keys()
  await Promise.all(names.map((name) => caches.delete(name)))
  return names.length
}

// Unregistering leaves the next load to fetch a fresh bundle. Registration is manual in
// @/helpers/pwa (vite-plugin-pwa is configured with injectRegister: false), so there is no
// framework call to undo — the registration object is the whole story.
async function unregisterServiceWorkers(): Promise<number> {
  if (typeof navigator === 'undefined' || !('serviceWorker' in navigator)) return 0
  const registrations = await navigator.serviceWorker.getRegistrations()
  await Promise.all(registrations.map((registration) => registration.unregister()))
  return registrations.length
}

/**
 * Performs a reset and returns a one-line report for the panel.
 *
 * Returns rather than logs, because a button that appears to do nothing is worse than no button:
 * "3 datasets, 4 caches, sw off" is the difference between "it worked" and "did I click it?".
 */
export async function resetLocalState(target: ResetTarget): Promise<string> {
  if (target === 'onboarding') {
    useOnboardingStore().reset()
    return 'onboarding reset — navigate to see the gate redirect'
  }

  if (target === 'session') {
    await useSessionStore().logout()
    return 'logged out'
  }

  const datasets = await clearDatasets()
  const cacheCount = await clearCacheStorage()
  const track = await deleteTrackDb()
  const workers = await unregisterServiceWorkers()
  forget(RUNTIME_MIRROR_KEYS)

  const report =
    `${datasets.length} datasets, ${cacheCount} caches, ` +
    `track ${track ? 'deleted' : 'kept (open elsewhere?)'}, ${workers} sw`

  if (target === 'caches') return report

  // 'everything': the above plus identity and dev state, then a reload so nothing in memory
  // survives to contradict what is now on disk.
  useOnboardingStore().reset()
  try {
    await useSessionStore().logout()
  } catch {
    // Offline logout still clears the local identity; nothing to do here.
  }
  clearDevOverrides()
  window.location.reload()
  return report
}
