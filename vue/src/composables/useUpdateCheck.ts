import { useFreshnessLoop, type FreshnessTarget } from '@/composables/useFreshnessLoop'
import { checkForUpdate } from '@/helpers/pwa'

// Ask periodically whether a new build is waiting (task 298).
//
// # The problem this fixes
//
// `registerSW` checks for a new service worker exactly once, at registration \u2014 per *document*. On iOS an
// installed PWA's document survives suspend/resume for a long time: task 280 measured 47 minutes on one
// document, across seven cycles including a 32-minute suspension. So without this, a device that stays
// open never learns a new build exists, and the only escape is the OS discarding the app. Found in the
// field on a device sitting on `main.85` while `main.87` was current.
//
// That makes this the app's **only route for shipping a fix during an event**. PRD 017 built an operator
// lever for load that works without a release, on the argument that "waiting for a redeploy to stop a
// load problem is not a plan"; the same argument applies with more force to a correctness bug at 02:00.
//
// # Why it reuses useFreshnessLoop
//
// Because it is the same question at a different cadence \u2014 "has something changed?", asked at the moments
// worth asking \u2014 and that composable already owns every answer this needs: check on foreground, check on
// an interval *while visible*, check on reconnect, stop entirely when hidden, and debounce repetition. A
// second timer here would have to re-derive all of it, and would be the third thing in this app polling
// on its own schedule, which is exactly what PRD 017 spent its effort removing.
//
// It is a **fourth** consumer of that convention (after contacts, the quiet prefetch, and the sync loop \u2014
// the first two of which were collapsed into the third), which is the first real evidence that the
// convention generalises rather than just being reusable in principle.
//
// # It never applies the update
//
// It only makes the banner appear. `registerType: 'prompt'` is deliberate and `UpdatePrompt` still waits
// for the user: reloading the app under a patrol mid-navigation would be worse than the bug being fixed.

/**
 * How often to ask, while the app is visible.
 *
 * Fifteen minutes: short enough that a fix shipped during an event reaches open devices within a
 * reasonable window, long enough to be free \u2014 a conditional request for one small file, so a few hundred
 * devices work out to well under one request per second.
 *
 * Deliberately **not** served from `/api/config`, unlike PRD 017's interval. That lever exists so an
 * operator can shed load during an event; this one addresses "we cannot ship a fix at all", and making it
 * remotely disableable would hand somebody a switch whose only effect is to reinstate the bug.
 */
const UPDATE_CHECK_SECONDS = 15 * 60

/**
 * A minimum gap between checks.
 *
 * Longer than the sync loop's 5 s, because this is a much less urgent question: unlock, glance, lock,
 * unlock should ask once, not four times. A minute is invisible to a user and removes the burst.
 */
const UPDATE_DEBOUNCE_SECONDS = 60

export interface UpdateCheckOptions {
  /** Injected for tests, which have no DOM. */
  target?: FreshnessTarget
  /** Overrides the interval. Tests only. */
  intervalSeconds?: number
  /** Overrides the debounce. Tests only. */
  debounceSeconds?: number
  /** Overrides what a check does. Tests only; production asks the service worker. */
  check?: () => Promise<void>
}

export function useUpdateCheck(options: UpdateCheckOptions = {}) {
  return useFreshnessLoop({
    check: options.check ?? checkForUpdate,
    intervalSeconds: options.intervalSeconds ?? UPDATE_CHECK_SECONDS,
    debounceSeconds: options.debounceSeconds ?? UPDATE_DEBOUNCE_SECONDS,
    target: options.target,
  })
}
