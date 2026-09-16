import { getCurrentScope, onScopeDispose } from 'vue'

// The shared freshness loop: "has this dataset changed?", asked at the moments worth asking
// (PRD 009 §6, task 190).
//
// Extracted from the contacts loop (task 162), which shipped first out of necessity and turned out
// to contain nothing contacts-specific except *what* to check. This file is the convention; the
// per-dataset wrapper supplies the check.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// THE CONVENTION, for the next dataset that needs to stay current during an event
// ────────────────────────────────────────────────────────────────────────────────────────────────
//
// 1. **A separate, cheap version endpoint** — not a poll of the payload. It returns a small opaque
//    version for *the caller's permitted set only*, answered from a projection read, and is
//    `ETag`-able. `GET /api/contacts/version` is the reference. Sizing matters: a few hundred
//    devices ask every 60 s while the app is open, which is this API's only continuous during-race
//    traffic and lands on the same BFF as position reporting.
//
// 2. **The version travels in the JSON body**, not in a header. Deliberate: `fetchWrapper` does not
//    expose response headers, so a header-only version would force every consumer to bypass it.
//    ETags stay on the response for the browser's own conditional requests.
//
// 3. **Three trigger points, and no more** — foreground (which includes mount), an interval while
//    visible, and `online`. Anything else is either a duplicate of these or a background timer on a
//    phone in a pocket.
//
// 4. **The interval is served, not built in.** `contactsPollSeconds` comes from `/api/config`. The
//    reason is load, not tidiness: if a few hundred devices cost more than expected, the interval
//    has to be widenable *during* an event without shipping a release. Follow the naming and add
//    another value rather than reusing this one — two datasets polling on one number cannot be
//    tuned apart, and they will not have the same cost.
//
// 5. **Zero disables the interval and nothing else.** Foreground and reconnect checks keep running,
//    so an operator reducing load at 02:00 cannot accidentally turn "poll less" into "stop
//    updating". This is the distinction they would get wrong, so it is the one with its own test.
//
// 6. **Metadata propagates ahead of images.** A corrected phone number arriving a minute before the
//    new portrait is fine; the reverse is not.
//
// 7. **Push is not an option for invalidation.** iOS 16.4+ requires *every* web push to raise a
//    user-visible notification — `public/push-sw.js` always calls `showNotification` — so using it
//    here would either buzz every crew member's phone over a corrected phone number or get the
//    permission revoked. There is no silent data push on our baseline.
//
// 8. **Replace, do not merge, when the answer changes** (PRD 009 §6, task 191). A dataset small
//    enough for one payload should be replaced wholesale, so a field the server stops sending stops
//    existing on the device. A delta needs explicit tombstones or the server's purge is decorative.

/**
 * The browser surface this loop needs, as an argument rather than a global.
 *
 * Same reasoning as `helpers/platform.ts` and the contacts store's storage seam, and the reason
 * `vitest.config.ts` can run in `node`: the loop is a decision about *when* to check, which is worth
 * testing without a DOM.
 */
export interface FreshnessTarget {
  isVisible(): boolean
  onVisibilityChange(handler: () => void): () => void
  onOnline(handler: () => void): () => void
  setInterval(handler: () => void, ms: number): number
  clearInterval(id: number): void
  /**
   * The clock, injected for the same reason as everything else here: the debounce is a decision
   * about elapsed time, and a test that has to actually wait five seconds to assert a five-second
   * window is a test nobody will keep.
   */
  now(): number
}

export interface FreshnessLoopSpec {
  /** The check itself. Must not throw: a failed check keeps the cached copy and records why. */
  check: () => Promise<void> | void
  /** Seconds between checks while visible. Zero or less disables the interval only. */
  intervalSeconds: number
  /**
   * Minimum seconds between checks. Zero or less disables the debounce.
   *
   * Guards *repetition*, where `checking` guards *overlap* — a distinction that sounds academic and
   * is not: unlock, glance at the map, lock, unlock again is how this app is used while walking, and
   * each of those unlocks is a completed, non-overlapping, entirely redundant check.
   *
   * Deliberately defaulted to zero rather than to a config value. The loop is the mechanism; how
   * often a given dataset is worth asking about is policy, and policy belongs to the caller that
   * knows which dataset it is. `useSyncLoop` passes the served value.
   */
  debounceSeconds?: number
  /**
   * Optional gate, consulted before every check.
   *
   * For a dataset the current user has no business fetching at all — a spejder has no contacts
   * pane — returning false is cheaper and quieter than letting the request 403 sixty times an hour.
   */
  enabled?: () => boolean
  target?: FreshnessTarget
}

/** Options for a single check. */
export interface CheckOptions {
  /**
   * Ignore the debounce.
   *
   * For a user-initiated refresh only. The user has asked, and a control that visibly does nothing
   * is worse than the request it saves. Everything else — the overlap guard, the visibility guard,
   * `enabled` — still applies: forcing means "do not tell me it is too soon", not "fetch data this
   * user may not have".
   */
  force?: boolean
}

export function browserFreshnessTarget(): FreshnessTarget {
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
    now: () => Date.now(),
  }
}

/**
 * Start a freshness loop and return a stop function.
 *
 * Scope-bound when there is a scope, so leaving the surface stops the traffic. That is the cheapest
 * possible scope and it means a user who never opens a pane generates nothing for it.
 */
export function useFreshnessLoop(spec: FreshnessLoopSpec) {
  const target = spec.target ?? browserFreshnessTarget()

  let timer: number | null = null
  let stopped = false
  // Guards against overlapping checks: a foreground event landing on top of an interval tick would
  // otherwise fire two requests, and on a slow link the second is pure waste.
  let checking = false
  // Guards against *repeated* checks, which the overlap guard cannot see because they do not
  // overlap. Null until the first check, so the mount check is never debounced away — nothing
  // precedes it, and a loop that did nothing when it started would be a strange thing to own.
  let lastCheckedAt: number | null = null

  // Mutable, because both values are served by the server and may change *while the loop runs* — the
  // 02:00 load lever (PRD 017). Seeded from the spec so a caller with no served value still works.
  let intervalSeconds = spec.intervalSeconds
  let debounceMs = Math.max(0, (spec.debounceSeconds ?? 0) * 1000)

  function tooSoon(): boolean {
    if (debounceMs <= 0 || lastCheckedAt === null) return false
    return target.now() - lastCheckedAt < debounceMs
  }

  async function check(options: CheckOptions = {}) {
    if (stopped || checking) return
    // A hidden document must not generate traffic even if a timer somehow survives — belt and braces
    // around the visibility handling below.
    if (!target.isVisible()) return
    if (spec.enabled && !spec.enabled()) return
    if (!options.force && tooSoon()) return

    // Stamped before the check rather than after, so the window is "time since we last asked" and a
    // slow round trip cannot extend it into a second skipped check.
    lastCheckedAt = target.now()
    checking = true
    try {
      await spec.check()
    } finally {
      checking = false
    }
  }

  function startTimer() {
    // Zero or less is the operator's kill switch for the interval. Foreground and reconnect checks
    // keep working, so "reduce load" can never silently become "stop updating".
    if (timer !== null || intervalSeconds <= 0) return
    timer = target.setInterval(() => void check(), intervalSeconds * 1000)
  }

  function stopTimer() {
    if (timer === null) return
    target.clearInterval(timer)
    timer = null
  }

  /**
   * Adopt a new interval, served by the server.
   *
   * Restarts the timer rather than leaving it on the old period, which is the whole point: an
   * operator widening the interval during an event needs it to take effect now, on a device that may
   * not be reloaded for hours. Zero stops the timer and leaves every other trigger alone.
   */
  function setIntervalSeconds(seconds: number) {
    if (seconds === intervalSeconds) return
    intervalSeconds = seconds
    stopTimer()
    if (target.isVisible()) startTimer()
  }

  /** Adopt a new debounce window, served by the server. Takes effect on the next check. */
  function setDebounceSeconds(seconds: number) {
    debounceMs = Math.max(0, seconds * 1000)
  }

  const offVisibility = target.onVisibilityChange(() => {
    if (target.isVisible()) {
      startTimer()
      void check()
    } else {
      // Hidden: stop entirely rather than lengthening the interval. A phone in a pocket has nobody
      // reading anything.
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

  // Mounting is a foregrounding: check now, then keep it current.
  if (target.isVisible()) {
    startTimer()
    void check()
  }

  // Checked rather than try/caught: `onScopeDispose` warns instead of throwing when there is no
  // scope, so a try/catch would still print. Tests construct the loop directly and own `stop`.
  if (getCurrentScope()) onScopeDispose(stop)

  return { stop, check, setIntervalSeconds, setDebounceSeconds }
}
