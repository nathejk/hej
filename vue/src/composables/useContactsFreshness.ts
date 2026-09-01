import { contactsPollSeconds } from '@/config/runtime'
import {
  useFreshnessLoop,
  type FreshnessTarget,
  type FreshnessLoopSpec,
} from '@/composables/useFreshnessLoop'
import { useContactsStore } from '@/stores/contacts.store'

// The contacts freshness loop (PRD 007 §6/§8, task 162).
//
// Keeps the cached directory current during the event: a change made upstream is visible immediately
// when the app comes to the foreground, and within ~60 s while it is open.
//
// **Now a thin wrapper.** The loop itself moved to `useFreshnessLoop` in task 190 — it contained
// nothing contacts-specific except *what* to check, and PRD 009 asked for the pattern to be the
// convention rather than one feature's private arrangement. The reasoning for the three trigger
// points, the served interval, the zero-disables-the-interval rule and why push cannot be used lives
// there. What stays here is this dataset's own two decisions:
//
//   - the check is `refreshIfStale`, which asks the cheap version endpoint and only refetches the
//     manifest when the answer differs;
//   - a role without the pane is skipped entirely, rather than being allowed to 403 sixty times an
//     hour against an endpoint that will never say yes.

/** Options exist for the tests; production uses the defaults. */
export interface FreshnessLoopOptions {
  /** Override the poll interval in seconds. Defaults to the runtime config value. */
  intervalSeconds?: number
  /** Injected for tests, which have no DOM. Defaults to `document`/`window`. */
  target?: FreshnessTarget
}

export type { FreshnessTarget, FreshnessLoopSpec }

/**
 * Starts the loop and returns a stop function.
 *
 * Called from the contacts pane, so it exists only while somebody is on it — the cheapest possible
 * scope, and it means no traffic at all from a user who never opens the pane. The pane also gets the
 * initial check for free, since starting counts as foregrounding.
 */
export function useContactsFreshness(options: FreshnessLoopOptions = {}) {
  const store = useContactsStore()

  return useFreshnessLoop({
    // Never throws (see the store); a failed check leaves the copy and records staleness. The
    // boolean it returns — "did we actually refetch" — is for callers that want to log; the loop
    // does not care, so it is dropped here rather than widening the shared spec's signature.
    check: async () => {
      await store.refreshIfStale()
    },
    enabled: () => !store.forbidden,
    intervalSeconds: options.intervalSeconds ?? contactsPollSeconds.value,
    target: options.target,
  })
}
