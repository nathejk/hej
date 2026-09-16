import { describe, expect, it } from 'vitest'

import {
  useFreshnessLoop,
  type FreshnessTarget,
} from '@/composables/useFreshnessLoop'

// A scriptable browser *and clock*, so the debounce window can be asserted without waiting for it.
// Same reasoning as `useContactsFreshness.spec.ts`'s fake, plus `advance`.
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
