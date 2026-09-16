import { describe, expect, it, vi } from 'vitest'

// `helpers/pwa` imports `virtual:pwa-register`, which only exists when Vite's PWA plugin is in the
// pipeline — it is not there under vitest, and `vitest.config.ts` keeps the environment `node`
// deliberately. Mocked at the module seam rather than worked around: every test below injects its own
// `check`, so the real one is never wanted here, and this file is about *when* the question is asked.
vi.mock('@/helpers/pwa', () => ({ checkForUpdate: vi.fn(async () => {}) }))

import { useUpdateCheck } from '@/composables/useUpdateCheck'
import type { FreshnessTarget } from '@/composables/useFreshnessLoop'

// The bug this guards (task 298): the app checked for a new build once per document load, and on iOS a
// document survives for hours across suspend/resume — so a device that stayed open never learned a fix
// existed. These tests are about *when* the question gets asked, since that was the entire defect.

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
      return () => {}
    },
    onOnline(handler) {
      onlineHandlers.push(handler)
      return () => {}
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

async function flush() {
  for (let i = 0; i < 4; i++) await Promise.resolve()
}

describe('useUpdateCheck', () => {
  it('asks once on start', () => {
    const check = vi.fn(async () => {})
    const target = fakeTarget()
    const loop = useUpdateCheck({ target, check })

    expect(check).toHaveBeenCalledTimes(1)
    loop.stop()
  })

  // The defect itself: an app that stays open must keep asking. On iOS "stays open" was measured at 47
  // minutes across seven suspend/resume cycles (task 280).
  it('keeps asking on an interval while the app stays open', async () => {
    const check = vi.fn(async () => {})
    const target = fakeTarget()
    const loop = useUpdateCheck({ target, check, intervalSeconds: 900, debounceSeconds: 60 })
    await flush()

    expect(target.intervalMs()).toBe(900_000)

    target.advance(900_000)
    target.tick()
    await flush()
    target.advance(900_000)
    target.tick()
    await flush()

    expect(check).toHaveBeenCalledTimes(3)
    loop.stop()
  })

  // Coming out of a pocket is exactly when a waiting fix should be noticed.
  it('asks on foreground', async () => {
    const check = vi.fn(async () => {})
    const target = fakeTarget()
    const loop = useUpdateCheck({ target, check, debounceSeconds: 60 })
    await flush()

    target.advance(120_000)
    target.setVisible(false)
    target.setVisible(true)
    await flush()

    expect(check).toHaveBeenCalledTimes(2)
    loop.stop()
  })

  it('asks on reconnect, since a check needs the network', async () => {
    const check = vi.fn(async () => {})
    const target = fakeTarget()
    const loop = useUpdateCheck({ target, check, debounceSeconds: 60 })
    await flush()

    target.advance(120_000)
    target.goOnline()
    await flush()

    expect(check).toHaveBeenCalledTimes(2)
    loop.stop()
  })

  // A phone in a pocket has nobody to show a banner to.
  it('does not ask while hidden', () => {
    const check = vi.fn(async () => {})
    const target = fakeTarget()
    const loop = useUpdateCheck({ target, check })

    target.setVisible(false)
    expect(target.activeTimers()).toBe(0)
    target.tick()

    expect(check).toHaveBeenCalledTimes(1) // the mount check only
    loop.stop()
  })

  // Unlock, glance, lock, unlock should ask once, not four times — this question is far less urgent than
  // the data check, so its debounce is a minute rather than five seconds.
  it('debounces a burst of foregrounds', async () => {
    const check = vi.fn(async () => {})
    const target = fakeTarget()
    const loop = useUpdateCheck({ target, check, debounceSeconds: 60 })
    await flush()

    for (let i = 0; i < 3; i++) {
      target.advance(2_000)
      target.setVisible(false)
      target.setVisible(true)
      await flush()
    }

    expect(check).toHaveBeenCalledTimes(1)
    loop.stop()
  })

  // A failed check is a non-event: offline, or the worker not registered yet. It must not stop the loop.
  //
  // `checkForUpdate` catches internally, so this cannot happen in production — but the loop containing it
  // is what keeps a future consumer's bug from becoming an unhandled rejection with no stack naming the
  // loop. The console error is expected here.
  it('survives a failing check and asks again', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    let calls = 0
    const check = vi.fn(async () => {
      calls++
      if (calls === 1) throw new Error('offline')
    })
    const target = fakeTarget()
    const loop = useUpdateCheck({ target, check, intervalSeconds: 900, debounceSeconds: 0 })
    await flush()

    target.tick()
    await flush()

    expect(calls).toBe(2)
    expect(consoleError).toHaveBeenCalled()
    loop.stop()
    consoleError.mockRestore()
  })
})
