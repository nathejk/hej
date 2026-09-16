import { HttpError } from '@/helpers'
import { fetchWrapper } from '@/helpers'
import {
  useFreshnessLoop,
  type CheckOptions,
  type FreshnessTarget,
} from '@/composables/useFreshnessLoop'
import { reportDirectory } from '@/helpers/offline/reporters'
import { heldTileAreaVersion, tileAreaIsStale } from '@/helpers/offline/tileAreaVersion'
import { useCheckpointsStore } from '@/stores/checkpoints.store'
import { useContactsStore } from '@/stores/contacts.store'
import { useHandoutsStore } from '@/stores/handouts.store'
import { useOfflineStore } from '@/stores/offline.store'
import { useProfileStore } from '@/stores/profile.store'
import { useScansStore } from '@/stores/scans.store'
import { useSessionStore } from '@/stores/session.store'
import type { SyncDataset } from '@/stores/syncVersions'

// The app's single freshness loop (PRD 017, task 286).
//
// One request per foreground asks whether anything this device holds has changed, and only the
// datasets whose answer differs are refetched. It replaces two loops that both checked the contacts
// directory (`useContactsFreshness`, `useQuietPrefetch`) and a mount-time scan fetch that never
// checked again — see task 288.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// WHAT THE SERVER'S ANSWER MEANS
// ────────────────────────────────────────────────────────────────────────────────────────────────
//
// **A version present and different** → refetch that dataset. Present and equal → do nothing; this is
// the common case and it costs no further request.
//
// **A key absent** → this user may not hold that dataset, so do not request it. A spejder gets no
// `contacts` key. This is deliberately the server's decision rather than the client's role table:
// when the two disagreed, the client asked anyway and collected a 403 per foreground.
//
// **A name in `unavailable`** → the server could not derive that version this time. Keep the cached
// copy, do not refetch, and ask again on the next check. This must never be treated as absence: a
// transient projection error read as a permission decision would stop the device asking, and nothing
// would ever tell it otherwise.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// FAILURE IS PER-DATASET
// ────────────────────────────────────────────────────────────────────────────────────────────────
//
// Each refresh is awaited independently and its failure is contained. One store failing must not stop
// the other four applying, and must not stop the loop — the alternative is that one broken dataset
// freezes every dataset on the device, which is strictly worse than the problem.
//
// Refetches may also *apply* out of order relative to one another, so nothing may assume that (say)
// checkpoints and handouts are mutually consistent within a single check.

/** The sync endpoint's response. Deliberately small; see `sync.go` on why it stays that way. */
interface SyncResponse {
  versions?: Partial<Record<SyncDataset, string>> | null
  unavailable?: string[] | null
  interval_seconds?: number
  debounce_seconds?: number
}

export interface SyncLoopOptions {
  /** Injected for tests, which have no DOM. */
  target?: FreshnessTarget
  /** Overrides the served interval. Tests only; production takes the server's value. */
  intervalSeconds?: number
  /** Overrides the served debounce. Tests only. */
  debounceSeconds?: number
}

/**
 * What a check turned out to be, for a caller that has to say something about it.
 *
 * The loop itself does not care — it checks and moves on — but a user who tapped a refresh button is
 * owed an answer, including in the boring case. `'unchanged'` is that case, and it is the *common*
 * one: a control that stays silent when nothing changed reads as broken and gets tapped again.
 */
export type SyncStatus =
  /** At least one dataset was refetched. */
  | 'refreshed'
  /** The check succeeded and everything we hold is current. */
  | 'unchanged'
  /** We could not ask: no signal, or the request never completed. */
  | 'offline'
  /** We asked and the server failed. Different from `offline` because "try again with signal" is
   * advice, and giving it to someone who has signal is a lie they will act on. */
  | 'error'
  /** The session is gone; the loop has stopped. */
  | 'unauthenticated'
  /** Nothing ran — a check was already in flight, or nobody is signed in. */
  | 'skipped'

export interface SyncOutcome {
  status: SyncStatus
  /** Which datasets were actually refetched. Empty unless `status` is `'refreshed'`. */
  refreshed: SyncDataset[]
}

/**
 * A dataset's refresh, keyed by the name the server uses.
 *
 * A `Record` over the `SyncDataset` union rather than a lookup by string, so a dataset the server can
 * report and the client forgot to handle is a **type error** rather than a dataset that silently never
 * refreshes — which would look exactly like the feature working.
 */
type Dispatch = Record<SyncDataset, (version: string) => Promise<boolean>>

function dispatchTable(): Dispatch {
  return {
    contacts: async (version) => {
      const refreshed = await useContactsStore().refreshIfVersionDiffers(version)
      // Keeps the offline-readiness screen's "last synced" honest. Only after a refresh: reporting on
      // every check would claim a sync happened when nothing was fetched.
      if (refreshed) reportDirectory()
      return refreshed
    },
    profile: (version) => useProfileStore().refreshIfVersionDiffers(version),
    scans: (version) => useScansStore().refreshIfVersionDiffers(version),
    handouts: (version) => useHandoutsStore().refreshIfVersionDiffers(version),
    checkpoints: (version) => useCheckpointsStore().refreshIfVersionDiffers(version),
    // The race area has no cached copy to refresh: it is fetched on demand when a bulk tile download
    // starts. So a changed version is not a refetch but a *fact about the tiles* — the event's area has
    // moved beyond what this device downloaded — and it is reported to the readiness surface for the user
    // to act on. Never a silent re-download: a few hundred megabytes stays somebody's decision (task 294).
    race_area: async (version) => {
      const stale = tileAreaIsStale(heldTileAreaVersion(), version)
      useOfflineStore().report('tiles', { updateAvailable: stale })
      // Always false: nothing was refreshed, and saying otherwise would make the manual refresh control
      // claim it fetched something when all it did was notice.
      return false
    },
  }
}

/**
 * Start the app's freshness loop and return `{ stop, check }`.
 *
 * `check` is exposed forced-capable so the manual refresh control (task 282) uses this same path
 * rather than growing a second way to reach `/api/sync`.
 */
export function useSyncLoop(options: SyncLoopOptions = {}) {
  const session = useSessionStore()
  const dispatch = dispatchTable()

  // The served values, adopted from each response. Held outside the loop so a change takes effect on
  // the next check without a reload — the 02:00 lever (task 289).
  let intervalSeconds = options.intervalSeconds ?? 60
  let debounceSeconds = options.debounceSeconds ?? 5

  // Set when the session has gone: a 401 must not be retried on every foreground for the rest of the
  // app's life. The existing auth handling owns the redirect; this only stops the traffic.
  let unauthenticated = false

  // The last completed check's result, plus a counter so a caller can tell "nothing changed" from
  // "the check never ran" — two answers a refresh button must not confuse, since the second is not
  // something to reassure anybody about.
  let outcome: SyncOutcome = { status: 'skipped', refreshed: [] }
  let completed = 0

  const loop = useFreshnessLoop({
    // No point asking on behalf of nobody. Also covers the moment between app start and a restored
    // session, where a check would 401 and then permanently disable itself.
    enabled: () => Boolean(session.user?.userId) && !unauthenticated,
    intervalSeconds,
    debounceSeconds,
    target: options.target,
    check: async () => {
      let response: SyncResponse
      try {
        response = await fetchWrapper.get<SyncResponse>('/api/sync')
      } catch (err) {
        if (err instanceof HttpError && err.status === 401) {
          unauthenticated = true
          outcome = { status: 'unauthenticated', refreshed: [] }
        } else {
          // A non-event for the loop: offline, a hiccup, a 500. The cached copies stay and the panes'
          // own staleness affordances say so (PRD 009). The two are kept apart only because the
          // refresh control says one of them out loud, and "no signal" is the wrong thing to tell
          // someone whose signal is fine.
          outcome = {
            status: err instanceof HttpError ? 'error' : 'offline',
            refreshed: [],
          }
        }
        completed += 1
        return
      }

      if (typeof response.interval_seconds === 'number') {
        intervalSeconds = response.interval_seconds
        loop.setIntervalSeconds(intervalSeconds)
      }
      if (typeof response.debounce_seconds === 'number') {
        debounceSeconds = response.debounce_seconds
        loop.setDebounceSeconds(debounceSeconds)
      }

      const versions = response.versions ?? {}
      // Named, so it can be excluded explicitly rather than by happening not to be in `versions`.
      const unavailable = new Set(response.unavailable ?? [])

      const names = Object.keys(versions) as SyncDataset[]
      const refreshed: SyncDataset[] = []
      await Promise.all(
        names.map(async (name) => {
          const version = versions[name]
          if (version === undefined) return
          if (unavailable.has(name)) return
          const refresh = dispatch[name]
          // A dataset this build does not know about. Ignored rather than treated as an error: a
          // server ahead of an installed client is a normal state for a PWA, and the datasets this
          // build *does* know must keep working.
          if (!refresh) return
          try {
            if (await refresh(version)) refreshed.push(name)
          } catch {
            // Contained on purpose. Every store promises not to throw, so reaching this is a bug
            // rather than a network condition — but one dataset's bug must not freeze the other four.
          }
        }),
      )

      outcome = {
        status: refreshed.length > 0 ? 'refreshed' : 'unchanged',
        refreshed,
      }
      completed += 1
    },
  })

  /**
   * Run a check now, on a user's behalf, and report what happened.
   *
   * Forced, so the debounce does not swallow a tap the user can see they made. The overlap guard still
   * applies, and a check dropped by it reports `'skipped'` rather than borrowing the previous check's
   * answer — telling someone "everything is up to date" on the strength of a check that never ran is
   * exactly the kind of confident wrong answer this feature is meant to remove.
   */
  async function refreshNow(): Promise<SyncOutcome> {
    // Answered without a check, because the loop is gated off and would otherwise report `'skipped'` —
    // i.e. "already refreshing" — to somebody whose session has expired. Wrong, and wrong in the
    // direction that keeps them tapping.
    if (unauthenticated) return { status: 'unauthenticated', refreshed: [] }

    const before = completed
    await loop.check({ force: true })
    if (completed === before) return { status: 'skipped', refreshed: [] }
    return outcome
  }

  const api = {
    stop: () => {
      active = null
      loop.stop()
    },
    check: (checkOptions?: CheckOptions) => loop.check(checkOptions),
    refreshNow,
  }

  // Registered so a control anywhere in the app can reach *this* loop rather than starting a second
  // one. A module-level reference rather than provide/inject because the loop is a singleton by
  // design (PRD 017 §6: "exactly one app-level loop"), and injection would let a stray provider
  // create a second silently.
  active = api

  return api
}

let active: { refreshNow(): Promise<SyncOutcome> } | null = null

/**
 * Run the app's sync check on a user's behalf.
 *
 * For the manual refresh control (task 282). Answers `'skipped'` when no loop is running — which is
 * the honest answer, and better than starting one on the spot: a loop created by a button press would
 * live outside the app's lifecycle and never be stopped.
 */
export async function refreshNow(): Promise<SyncOutcome> {
  if (!active) return { status: 'skipped', refreshed: [] }
  return await active.refreshNow()
}
