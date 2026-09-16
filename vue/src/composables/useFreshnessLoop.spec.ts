import { describe, expect, it } from 'vitest'

import {
  useFreshnessLoop,
  type FreshnessTarget,
} from '@/composables/useFreshnessLoop'

// A scriptable browser *and clock*, so the debounce window can be asserted without waiting for it.
// Plus `advance`, `intervalMs` and `activeTimers`, which the trigger-point tests below need.
function fakeTarget() {
  let visible = true
  let clock = 1_000_000
  const visibilityHandlers: (() => void)[] = []
  const onlineHandlers: (() => void)[] = []
  const timers = new Map<number, { handler: () => void; ms: number }>()
  let nextId = 1

  const target: FreshnessTarget & {
    setVisible(v: boolean): void
    goOnline(): void
    tick(): void
    advance(ms: number): void
    activeTimers(): number
    intervalMs(): number | null
  } = {
    isVisible: () => visible,
    onVisibilityChange(handler) {
      visibilityHandlers.push(handler)
      return () => {
        const i = visibilityHandlers.indexOf(handler)
        if (i >= 0) visibilityHandlers.splice(i, 1)
      }
    },
    onOnline(handler) {
      onlineHandlers.push(handler)
      return () => {
        const i = onlineHandlers.indexOf(handler)
        if (i >= 0) onlineHandlers.splice(i, 1)
      }
    },
    setInterval(handler, ms) {
      const id = nextId++
      timers.set(id, { handler, ms })
      return id
    },
    clearInterval(id) {
      timers.delete(id)
    },
    now: () => clock,

    setVisible(v) {
      visible = v
      visibilityHandlers.forEach((h) => h())
    },
    goOnline() {
      onlineHandlers.forEach((h) => h())
    },
    tick() {
      timers.forEach((t) => t.handler())
    },
    advance(ms) {
      clock += ms
    },
    activeTimers: () => timers.size,
    intervalMs: () => [...timers.values()][0]?.ms ?? null,
  }
  return target
}

// The loop suppresses overlapping checks and the check is async, so a trigger fired in the same
// synchronous turn as the previous one is *correctly* dropped. Tests expecting a second check let
// the first settle first — as a real check would, since it takes a round trip.
async function flush() {
  await Promise.resolve()
  await Promise.resolve()
}

function countingLoop(
  target: ReturnType<typeof fakeTarget>,
  debounceSeconds: number,
  intervalSeconds = 60,
) {
  const calls = { n: 0 }
  const loop = useFreshnessLoop({
    check: () => {
      calls.n += 1
    },
    intervalSeconds,
    debounceSeconds,
    target,
  })
  return { loop, calls }
}

describe('useFreshnessLoop debounce', () => {
  // The case this exists for: unlock, glance, lock, unlock is the map's normal rhythm, and each of
  // those foregrounds is a completed, non-overlapping, entirely redundant check.
  it('skips a repeated check inside the window', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 5)
    expect(calls.n).toBe(1)
    await flush()

    target.advance(1_000)
    target.setVisible(false)
    target.setVisible(true)
    await flush()

    expect(calls.n).toBe(1)
    loop.stop()
  })

  it('allows a check once the window has passed', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 5)
    await flush()

    target.advance(5_000)
    target.setVisible(false)
    target.setVisible(true)
    await flush()

    expect(calls.n).toBe(2)
    loop.stop()
  })

  // Nothing precedes the mount check, so there is nothing for it to be too soon after.
  it('never debounces the check on start', () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 60)
    expect(calls.n).toBe(1)
    loop.stop()
  })

  it('debounces interval ticks and reconnects too', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 5)
    await flush()

    target.advance(1_000)
    target.tick()
    await flush()
    target.goOnline()
    await flush()
    expect(calls.n).toBe(1)

    target.advance(9_000)
    target.tick()
    await flush()
    expect(calls.n).toBe(2)
    loop.stop()
  })

  it('treats zero and negative debounce as disabled', async () => {
    for (const debounce of [0, -5]) {
      const target = fakeTarget()
      const { loop, calls } = countingLoop(target, debounce)
      await flush()

      target.tick()
      await flush()
      target.tick()
      await flush()

      expect(calls.n).toBe(3)
      loop.stop()
    }
  })
})

describe('useFreshnessLoop trigger points', () => {
  // These assertions came from `useContactsFreshness.spec.ts` (tasks 162/190), which tested the shared
  // loop through the one dataset that happened to use it. Task 288 removed that wrapper; the
  // behaviours it pinned are the loop's own, so they moved here rather than being deleted.

  it('checks immediately on start', () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 0)
    expect(calls.n).toBe(1)
    loop.stop()
  })

  it('checks on the interval while visible', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 0)
    await flush()

    target.tick()
    await flush()
    target.tick()
    await flush()

    expect(calls.n).toBe(3)
    expect(target.intervalMs()).toBe(60_000)
    loop.stop()
  })

  // A phone in a pocket has nobody reading anything, so it must generate no traffic at all.
  it('stops polling entirely when hidden', () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 0)
    expect(target.activeTimers()).toBe(1)

    target.setVisible(false)
    expect(target.activeTimers()).toBe(0)

    // And even a stray timer firing must not produce a request.
    target.tick()
    expect(calls.n).toBe(1)
    loop.stop()
  })

  // The case that matters most: someone opening the app wants what they are looking at to be current.
  it('checks immediately on foreground, and resumes polling', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 0)
    await flush()
    target.setVisible(false)

    target.setVisible(true)
    await flush()

    expect(calls.n).toBe(2)
    expect(target.activeTimers()).toBe(1)
    loop.stop()
  })

  it('checks on reconnect', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 0)
    await flush()

    target.goOnline()
    await flush()

    expect(calls.n).toBe(2)
    loop.stop()
  })

  // Zero is the operator's kill switch for the interval — but not for the app. Foregrounding must
  // still check, or "reduce load" silently becomes "stop updating".
  it('honours a disabled interval without disabling foreground checks', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 0, 0)

    expect(calls.n).toBe(1)
    expect(target.activeTimers()).toBe(0)
    await flush()

    target.setVisible(false)
    target.setVisible(true)
    await flush()
    expect(calls.n).toBe(2)

    target.goOnline()
    await flush()
    expect(calls.n).toBe(3)
    loop.stop()
  })

  it('treats a negative interval like zero', () => {
    const target = fakeTarget()
    const { loop } = countingLoop(target, 0, -30)
    expect(target.activeTimers()).toBe(0)
    loop.stop()
  })

  it('does not start when the document is hidden', () => {
    const target = fakeTarget()
    target.setVisible(false)
    const { loop, calls } = countingLoop(target, 0)

    expect(calls.n).toBe(0)
    expect(target.activeTimers()).toBe(0)
    loop.stop()
  })

  // Overlapping checks are pure waste on a slow link, which is the link this app runs on.
  it('does not run overlapping checks', async () => {
    const target = fakeTarget()
    const releasers: (() => void)[] = []
    let calls = 0
    const loop = useFreshnessLoop({
      check: () => {
        calls += 1
        return new Promise<void>((resolve) => releasers.push(resolve))
      },
      intervalSeconds: 60,
      target,
    })
    expect(calls).toBe(1)

    // Two more triggers while the first is still in flight.
    target.tick()
    target.goOnline()
    expect(calls).toBe(1)

    // Let the first finish; a later trigger works normally.
    releasers.shift()?.()
    await flush()
    target.tick()
    await flush()
    expect(calls).toBe(2)

    releasers.forEach((r) => r())
    loop.stop()
  })

  // A dataset the current user has no business fetching — the `enabled` gate.
  it('generates no traffic when disabled', () => {
    const target = fakeTarget()
    let calls = 0
    const loop = useFreshnessLoop({
      check: () => {
        calls += 1
      },
      intervalSeconds: 60,
      enabled: () => false,
      target,
    })

    expect(calls).toBe(0)
    target.tick()
    expect(calls).toBe(0)
    loop.stop()
  })

  it('stops cleanly, leaving no listeners or timers', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 0)
    await flush()
    const before = calls.n

    loop.stop()

    expect(target.activeTimers()).toBe(0)
    target.setVisible(true)
    target.goOnline()
    target.tick()
    await flush()
    expect(calls.n).toBe(before)
  })
})

describe('useFreshnessLoop served values', () => {
  // The 02:00 lever (PRD 017): a widened interval must take effect on a device that may not be
  // reloaded for hours, so the timer is restarted rather than left running on the old period.
  it('restarts the timer when the interval changes', async () => {
    const target = fakeTarget()
    const { loop } = countingLoop(target, 0, 60)
    expect(target.intervalMs()).toBe(60_000)

    loop.setIntervalSeconds(300)
    expect(target.intervalMs()).toBe(300_000)
    expect(target.activeTimers()).toBe(1)
    loop.stop()
  })

  it('drops the timer when the interval becomes zero, and restores it when it returns', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 0, 60)
    // Let the mount check settle: while it is in flight the overlap guard would drop the foreground
    // check below, which is correct behaviour and not what this test is about.
    await flush()

    loop.setIntervalSeconds(0)
    expect(target.activeTimers()).toBe(0)

    // Still checks on foreground: zero disables the interval and nothing else.
    target.setVisible(false)
    target.setVisible(true)
    await flush()
    expect(calls.n).toBe(2)

    loop.setIntervalSeconds(60)
    expect(target.intervalMs()).toBe(60_000)
    loop.stop()
  })

  it('does not restart the timer when the interval is unchanged', () => {
    const target = fakeTarget()
    const { loop } = countingLoop(target, 0, 60)
    const before = target.activeTimers()

    loop.setIntervalSeconds(60)

    expect(target.activeTimers()).toBe(before)
    expect(target.intervalMs()).toBe(60_000)
    loop.stop()
  })

  it('adopts a new debounce window', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 0)
    await flush()

    loop.setDebounceSeconds(60)
    target.advance(1_000)
    target.tick()
    await flush()
    expect(calls.n).toBe(1)

    loop.setDebounceSeconds(0)
    target.tick()
    await flush()
    expect(calls.n).toBe(2)
    loop.stop()
  })
})

describe('useFreshnessLoop forced checks', () => {
  // The manual refresh control's whole value is settling the question "is this current?", so it must
  // not be silently swallowed by a window the user cannot see.
  it('bypasses the debounce', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 60)
    await flush()

    await loop.check({ force: true })
    expect(calls.n).toBe(2)

    await loop.check({ force: true })
    expect(calls.n).toBe(3)
    loop.stop()
  })

  it('does not bypass the overlap guard', async () => {
    const target = fakeTarget()
    // Held in an array rather than a `let`, because TypeScript cannot see the assignment inside the
    // promise executor and narrows a nullable binding to `never`.
    const releasers: (() => void)[] = []
    const releaseCheck = () => releasers.shift()?.()
    let calls = 0
    const loop = useFreshnessLoop({
      check: () => {
        calls += 1
        return new Promise<void>((resolve) => {
          releasers.push(resolve)
        })
      },
      intervalSeconds: 60,
      debounceSeconds: 5,
      target,
    })
    expect(calls).toBe(1)

    // Forcing while the first check is still in flight would put two identical requests on a slow
    // link, which is the waste the overlap guard exists to prevent — force does not license that.
    void loop.check({ force: true })
    expect(calls).toBe(1)

    releaseCheck()
    await flush()
    void loop.check({ force: true })
    expect(calls).toBe(2)

    releaseCheck()
    loop.stop()
  })

  it('does not bypass the enabled gate', async () => {
    const target = fakeTarget()
    let calls = 0
    const loop = useFreshnessLoop({
      check: () => {
        calls += 1
      },
      intervalSeconds: 60,
      debounceSeconds: 5,
      enabled: () => false,
      target,
    })

    await loop.check({ force: true })

    // Forcing means "do not tell me it is too soon", not "fetch something this user may not hold".
    expect(calls).toBe(0)
    loop.stop()
  })

  it('does not bypass a stopped loop', async () => {
    const target = fakeTarget()
    const { loop, calls } = countingLoop(target, 5)
    await flush()
    loop.stop()

    await loop.check({ force: true })
    expect(calls.n).toBe(1)
  })
})
