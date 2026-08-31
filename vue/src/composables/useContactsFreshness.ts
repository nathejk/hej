import { getCurrentScope, onScopeDispose } from 'vue'

import { contactsPollSeconds } from '@/config/runtime'
import { useContactsStore } from '@/stores/contacts.store'

// The contacts freshness loop (PRD 007 §6/§8, task 162).
//
// Keeps the cached directory current during the event: a change made upstream is visible
// immediately when the app comes to the foreground, and within ~60 s while it is open.
//
// # Why a pull, not a push
//
// iOS 16.4+ web push requires *every* push to raise a user-visible notification — there is no
// silent data push on our baseline, and `public/push-sw.js` reflects that by always calling
// `showNotification`. Using push for invalidation would either buzz every crew member's phone
// over a corrected phone number or risk them revoking the permission. So: a cheap pull, at
// moments that already exist.
//
// # Why the moments are what they are
//
//   - **foreground** — the case that matters most. Someone opening the pane wants it current,
//     and this is the only check that runs when the app was closed for hours.
//   - **reconnect** — changes may have been missed while offline, and coming back on to signal
//     is the first chance to catch up.
//   - **interval, only while visible** — the during-race requirement. Stopped the moment the
//     app is hidden, because a background timer on a phone in a pocket costs battery for
//     nothing: nobody is reading the pane.
//
// There is deliberately **no periodic background sync**: iOS does not offer it reliably, and a
// user who is not looking at the app does not need a fresh directory — they need the battery.

/** Options exist for the tests; production uses the defaults. */
export interface FreshnessLoopOptions {
  /** Override the poll interval in seconds. Defaults to the runtime config value. */
  intervalSeconds?: number
  /** Injected for tests, which have no DOM. Defaults to `document`/`window`. */
  target?: FreshnessTarget
}

/**
 * The browser surface this loop needs, as an argument rather than a global.
 *
 * Same reasoning as `helpers/platform.ts` and the contacts store's storage seam, and the
 * reason `vitest.config.ts` can run in `node`: the loop is a decision about *when* to check,
 * which is worth testing without a DOM.
 */
export interface FreshnessTarget {
  isVisible(): boolean
  onVisibilityChange(handler: () => void): () => void
  onOnline(handler: () => void): () => void
  setInterval(handler: () => void, ms: number): number
  clearInterval(id: number): void
}

function browserTarget(): FreshnessTarget {
  return {
    isVisible: () => typeof document === 'undefined' || document.visibilityState === 'visible',
    onVisibilityChange(handler) {
      document.addEventListener('visibilitychange', handler)
      return () => document.removeEventListener('visibilitychange', handler)
    },
    onOnline(handler) {
      window.addEventListener('online', handler)
      return () => window.removeEventListener('online', handler)
    },
    setInterval: (handler, ms) => window.setInterval(handler, ms),
    clearInterval: (id) => window.clearInterval(id),
  }
}

/**
 * Starts the freshness loop and returns a stop function.
 *
 * Called from the contacts pane, so the loop exists only while somebody is on it — the
 * cheapest possible scope, and it means no traffic at all from a user who never opens the
 * pane. The pane also gets the initial check for free, since starting counts as foregrounding.
 */
export function useContactsFreshness(options: FreshnessLoopOptions = {}) {
  const store = useContactsStore()
  const target = options.target ?? browserTarget()

  let timer: number | null = null
  let stopped = false
  // Guards against overlapping checks: a foreground event landing on top of an interval tick
  // would otherwise fire two requests, and on a slow link the second is pure waste.
  let checking = false

  async function check() {
    if (stopped || checking) return
    // A hidden document must not generate traffic, even if a timer somehow survives — belt and
    // braces around the visibility handling below.
    if (!target.isVisible()) return
    // Nothing to refresh for a role without the pane; the store has already cleared itself.
    if (store.forbidden) return

    checking = true
    try {
      // Never throws (see the store); a failed check leaves the copy and records staleness.
      await store.refreshIfStale()
    } finally {
      checking = false
    }
  }

  function startTimer() {
    const seconds = options.intervalSeconds ?? contactsPollSeconds.value
    // 0 or less is the operator's kill switch for the interval. Foreground and reconnect
    // checks keep working, so the pane still updates when someone opens it.
    if (timer !== null || seconds <= 0) return
    timer = target.setInterval(() => void check(), seconds * 1000)
  }

  function stopTimer() {
    if (timer === null) return
    target.clearInterval(timer)
    timer = null
  }

  const offVisibility = target.onVisibilityChange(() => {
    if (target.isVisible()) {
      startTimer()
      void check()
    } else {
      // Hidden: stop entirely rather than lengthening the interval. A phone in a pocket has
      // nobody reading the pane.
      stopTimer()
    }
  })

  const offOnline = target.onOnline(() => void check())

  function stop() {
    stopped = true
    stopTimer()
    offVisibility()
    offOnline()
  }

  // Mounting the pane is a foregrounding: check now, then keep it current.
  if (target.isVisible()) {
    startTimer()
    void check()
  }

  // Tied to the component's scope so leaving the pane stops the traffic.
  //
  // Checked rather than try/caught: `onScopeDispose` warns instead of throwing when there is no
  // scope, so a try/catch would still print. Tests construct the loop directly and own `stop`
  // themselves.
  if (getCurrentScope()) onScopeDispose(stop)

  return { stop, check }
}
