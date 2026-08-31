import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  useContactsFreshness,
  type FreshnessTarget,
} from '@/composables/useContactsFreshness'
import { useContactsStore } from '@/stores/contacts.store'

// A scriptable stand-in for the browser, so the loop's *timing decisions* can be tested
// without a DOM — see vitest.config.ts on why the environment stays `node`.
function fakeTarget() {
  let visible = true
  const visibilityHandlers: (() => void)[] = []
  const onlineHandlers: (() => void)[] = []
  const timers = new Map<number, { handler: () => void; ms: number }>()
  let nextId = 1

  const target: FreshnessTarget & {
    setVisible(v: boolean): void
    goOnline(): void
    tick(): void
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
    activeTimers: () => timers.size,
    intervalMs: () => [...timers.values()][0]?.ms ?? null,
  }
  return target
}

let refreshCalls = 0
let refreshResolvers: (() => void)[] = []
let blockRefresh = false

// Lets the in-flight check settle.
//
// Needed because the loop suppresses overlapping checks, and `refreshIfStale` is async: a
// trigger fired in the same synchronous turn as the previous one is *correctly* dropped. So
// each test that expects a second check has to let the first finish first — which is the
// production behaviour too, since a real check takes a network round trip.
async function flush() {
  await Promise.resolve()
  await Promise.resolve()
}

beforeEach(() => {
  setActivePinia(createPinia())
  refreshCalls = 0
  refreshResolvers = []
  blockRefresh = false

  const store = useContactsStore()
  // Stubbed at the store boundary: this file is about *when* a check happens, not what it
  // fetches — the store has its own tests for that.
  store.refreshIfStale = vi.fn(async () => {
    refreshCalls += 1
    if (blockRefresh) {
      await new Promise<void>((resolve) => refreshResolvers.push(resolve))
    }
    return false
  })
  // The store hydrates from storage on use; there is none in a node run.
  store.storage = null
})

describe('useContactsFreshness', () => {
  it('checks immediately on start', () => {
    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: 60 })

    expect(refreshCalls).toBe(1)
    loop.stop()
  })

  it('checks on the interval while visible', async () => {
    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: 60 })
    expect(refreshCalls).toBe(1)
    await flush()

    target.tick()
    await flush()
    target.tick()
    await flush()
    expect(refreshCalls).toBe(3)

    expect(target.intervalMs()).toBe(60_000)
    loop.stop()
  })

  // A phone in a pocket has nobody reading the pane, so it must generate no traffic at all.
  it('stops polling entirely when hidden', () => {
    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: 60 })
    expect(target.activeTimers()).toBe(1)

    target.setVisible(false)
    expect(target.activeTimers()).toBe(0)

    // And even a stray timer firing must not produce a request.
    target.tick()
    expect(refreshCalls).toBe(1)
    loop.stop()
  })

  // The case that matters most: someone opening the app wants the pane current.
  it('checks immediately on foreground, and resumes polling', async () => {
    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: 60 })
    await flush()
    target.setVisible(false)
    const before = refreshCalls

    target.setVisible(true)
    await flush()

    expect(refreshCalls).toBe(before + 1)
    expect(target.activeTimers()).toBe(1)
    loop.stop()
  })

  it('checks on reconnect', async () => {
    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: 60 })
    await flush()
    const before = refreshCalls

    target.goOnline()
    await flush()

    expect(refreshCalls).toBe(before + 1)
    loop.stop()
  })

  // 0 is the operator's kill switch for the interval — but not for the pane. Opening it must
  // still refresh, or a "reduce load" setting silently becomes "stop updating".
  it('honours a disabled interval without disabling foreground checks', async () => {
    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: 0 })

    expect(refreshCalls).toBe(1)
    expect(target.activeTimers()).toBe(0)
    await flush()

    target.setVisible(false)
    target.setVisible(true)
    await flush()
    expect(refreshCalls).toBe(2)

    target.goOnline()
    await flush()
    expect(refreshCalls).toBe(3)
    loop.stop()
  })

  it('treats a negative interval like zero', () => {
    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: -30 })
    expect(target.activeTimers()).toBe(0)
    loop.stop()
  })

  it('does not start when the document is hidden', () => {
    const target = fakeTarget()
    target.setVisible(false)
    const loop = useContactsFreshness({ target, intervalSeconds: 60 })

    expect(refreshCalls).toBe(0)
    expect(target.activeTimers()).toBe(0)
    loop.stop()
  })

  // Overlapping checks are pure waste on a slow link, which is the link this app runs on.
  it('does not run overlapping checks', async () => {
    blockRefresh = true
    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: 60 })
    expect(refreshCalls).toBe(1)

    // Two more triggers while the first is still in flight.
    target.tick()
    target.goOnline()
    expect(refreshCalls).toBe(1)

    // Let the first finish; a later trigger works normally.
    refreshResolvers.forEach((r) => r())
    await flush()
    blockRefresh = false
    target.tick()
    await flush()
    expect(refreshCalls).toBe(2)

    loop.stop()
  })

  it('generates no traffic for a role without the pane', () => {
    const store = useContactsStore()
    store.forbidden = true

    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: 60 })

    expect(refreshCalls).toBe(0)
    target.tick()
    expect(refreshCalls).toBe(0)
    loop.stop()
  })

  it('stops cleanly, leaving no listeners or timers', async () => {
    const target = fakeTarget()
    const loop = useContactsFreshness({ target, intervalSeconds: 60 })
    await flush()
    const before = refreshCalls

    loop.stop()

    expect(target.activeTimers()).toBe(0)
    target.setVisible(true)
    target.goOnline()
    target.tick()
    await flush()
    expect(refreshCalls).toBe(before)
  })
})
