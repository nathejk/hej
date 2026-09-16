import { HttpError } from '@/helpers'
import { fetchWrapper } from '@/helpers'
import {
  useFreshnessLoop,
  type CheckOptions,
  type FreshnessTarget,
} from '@/composables/useFreshnessLoop'
import { reportDirectory } from '@/helpers/offline/reporters'
import { useCheckpointsStore } from '@/stores/checkpoints.store'
import { useContactsStore } from '@/stores/contacts.store'
import { useHandoutsStore } from '@/stores/handouts.store'
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
    // The race area has no cached copy on the client: it is fetched on demand when a bulk tile
    // download starts. Nothing to refresh, so the version is accepted and ignored here — task 294
    // turns it into what it actually means, "more map is available to download".
    race_area: async () => false,
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
        if (err instanceof HttpError && err.status === 401) unauthenticated = true
        // Everything else is a non-event: offline, a hiccup, a 500. The cached copies stay and the
        // panes' own staleness affordances say so (PRD 009). Nothing to report here.
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
            await refresh(version)
          } catch {
            // Contained on purpose. Every store promises not to throw, so reaching this is a bug
            // rather than a network condition — but one dataset's bug must not freeze the other four.
          }
        }),
      )
    },
  })

  return {
    stop: loop.stop,
    check: (checkOptions?: CheckOptions) => loop.check(checkOptions),
  }
}
