// Prefetching the small datasets without asking (PRD 009 §7, task 194).
//
// # Why there is no prompt
//
// The contacts directory and its thumbnails are under ~1 MB for the largest role (tasks 078, 104).
// A screen asking permission to spend a megabyte is a screen that teaches people to tap past
// screens, and the one prompt that genuinely matters — the ~324 MB bulk map download — has to be
// believed when it appears. So the cheap data is simply fetched.
//
// # Why it runs at app level and not just in the pane
//
// Details and portraits churn hardest in the run-up, while people are still adding photographs and
// checking their own records. That makes the *first* sync the least accurate copy a device will ever
// hold, and it means the interesting work is the catching up — on a phone whose owner may not open
// `Kontakter` until the night of the race, or ever.
//
// `ContactsView` runs its own loop while somebody is on the pane (task 162). This is the other case:
// the device that never gets there.
//
// # Why it adds no continuous traffic
//
// Foreground and reconnect only — `intervalSeconds: 0`, which the shared loop reads as "keep the
// event-driven checks, drop the timer" (task 190). The during-race interval stays scoped to the pane,
// where somebody is actually reading the result. A phone in a pocket polling for a directory nobody
// is looking at spends battery and BFF capacity for nothing.
//
// # Why portraits are not prefetched in bulk
//
// They arrive with the rows that display them, and that is deliberate rather than unfinished. Early
// in the run-up most portraits are about to be replaced — a bulk fetch three weeks out would spend a
// participant's mobile data on bytes that are stale before the race. The index is what makes the pane
// usable offline; faces fill in as the pane is used, and `ContactRow` already distinguishes "no
// photo" from "not on this device" (task 169).

import { hasContactsPane } from '@/config/roles'
import { useFreshnessLoop, type FreshnessTarget } from '@/composables/useFreshnessLoop'
import { reportDirectory } from '@/helpers/offline/reporters'
import { useContactsStore } from '@/stores/contacts.store'
import { useSessionStore } from '@/stores/session.store'

export interface QuietPrefetchOptions {
  /** Injected for tests, which have no DOM. */
  target?: FreshnessTarget
}

/**
 * Fetch the cheap datasets if this user has them, and keep them current on foreground and reconnect.
 *
 * Returns the loop's `stop`, so the app can tear it down; in practice it lives as long as the app
 * does.
 */
export function useQuietPrefetch(options: QuietPrefetchOptions = {}) {
  const session = useSessionStore()
  const contacts = useContactsStore()

  return useFreshnessLoop({
    // Gated on **role**, not on letting the server refuse. A spejder has no contacts pane, so
    // without this every spejder device would ask for a directory the BFF will always deny, on
    // every foreground — a few hundred phones generating 403s all race for a pane they cannot open.
    //
    // `forbidden` is also checked, for the case the client's idea of the role is wrong: the BFF is
    // the authority, and once it has said no we stop asking.
    enabled: () =>
      Boolean(session.user?.userId) && hasContactsPane(session.user?.role) && !contacts.forbidden,
    check: async () => {
      // `refreshIfStale` fetches outright when nothing is held and otherwise costs one small version
      // request. So the first launch prefetches, and every launch after that is a cheap check.
      await contacts.refreshIfStale()
      reportDirectory()
    },
    // Deliberately no interval. See the note above.
    intervalSeconds: 0,
    target: options.target,
  })
}
