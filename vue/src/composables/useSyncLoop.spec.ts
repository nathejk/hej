import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { HttpError } from '@/helpers'
import { fetchWrapper } from '@/helpers'
import { useSyncLoop } from '@/composables/useSyncLoop'
import type { FreshnessTarget } from '@/composables/useFreshnessLoop'
import { useCheckpointsStore } from '@/stores/checkpoints.store'
import { useContactsStore } from '@/stores/contacts.store'
import { useHandoutsStore } from '@/stores/handouts.store'
import { useProfileStore } from '@/stores/profile.store'
import { useScansStore } from '@/stores/scans.store'
import { useSessionStore } from '@/stores/session.store'
import { useOfflineStore } from '@/stores/offline.store'
import { rememberTileAreaVersion } from '@/helpers/offline/tileAreaVersion'

// A scriptable browser and clock. Same seam as `useFreshnessLoop.spec.ts`; this file is about what the
// loop does with an *answer*, not about when it asks.
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
    intervalMs(): number | null
    activeTimers(): number
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
    intervalMs: () => [...timers.values()][0]?.ms ?? null,
    activeTimers: () => timers.size,
  }
  return target
}

async function flush() {
  for (let i = 0; i < 6; i++) await Promise.resolve()
}

let getMock: ReturnType<typeof vi.fn>
let refreshed: Record<string, string[]>

// The tile-area version lives in `localStorage`, which a node run does not have (see vitest.config.ts on
// why the environment stays `node`). Stubbed as a global rather than injected, so the loop exercises the
// same read path it uses in production.
function stubLocalStorage() {
  const backing = new Map<string, string>()
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: {
      getItem: (k: string) => backing.get(k) ?? null,
      setItem: (k: string, v: string) => void backing.set(k, v),
      removeItem: (k: string) => void backing.delete(k),
    },
  })
}

// Every store is stubbed at its `refreshIfVersionDiffers` boundary: this file is about dispatch, and
// the stores have their own tests for what a refresh does (task 287).
function stubStores() {
  refreshed = { contacts: [], profile: [], scans: [], handouts: [], checkpoints: [] }
  const record = (name: string) => async (version: string) => {
    refreshed[name].push(version)
    return true
  }
  useContactsStore().refreshIfVersionDiffers = vi.fn(record('contacts'))
  useProfileStore().refreshIfVersionDiffers = vi.fn(record('profile'))
  useScansStore().refreshIfVersionDiffers = vi.fn(record('scans'))
  useHandoutsStore().refreshIfVersionDiffers = vi.fn(record('handouts'))
  useCheckpointsStore().refreshIfVersionDiffers = vi.fn(record('checkpoints'))
}

beforeEach(() => {
  setActivePinia(createPinia())
  stubLocalStorage()
  getMock = vi.fn()
  fetchWrapper.get = getMock as never
  const session = useSessionStore()
  session.user = { userId: 'u-1', role: 'spejder' } as never
  stubStores()
})

const fullResponse = {
  versions: {
    contacts: 'c1',
    profile: 'p1',
    scans: 's1',
    handouts: 'h1',
    checkpoints: 'k1',
    race_area: 'r1',
  },
  unavailable: [],
  interval_seconds: 60,
  debounce_seconds: 5,
}

describe('useSyncLoop dispatch', () => {
  it('checks once on start and dispatches every dataset it was given', async () => {
    getMock.mockResolvedValue(fullResponse)
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(getMock).toHaveBeenCalledTimes(1)
    expect(getMock.mock.calls[0][0]).toBe('/api/sync')
    expect(refreshed.contacts).toEqual(['c1'])
    expect(refreshed.profile).toEqual(['p1'])
    expect(refreshed.scans).toEqual(['s1'])
    expect(refreshed.handouts).toEqual(['h1'])
    expect(refreshed.checkpoints).toEqual(['k1'])
    loop.stop()
  })

  // Absence is the server saying "you may not hold this". Asking anyway is what the old client-side
  // role table did, and it collected a 403 per foreground when the two disagreed.
  it('never touches a dataset whose key is absent', async () => {
    getMock.mockResolvedValue({ versions: { profile: 'p1' }, unavailable: [] })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(refreshed.profile).toEqual(['p1'])
    expect(refreshed.contacts).toEqual([])
    expect(refreshed.scans).toEqual([])
    loop.stop()
  })

  // The distinction the whole `unavailable` field exists for: a transient server failure must not be
  // read as a permission decision, or the device stops asking and nothing ever tells it otherwise.
  it('keeps the copy for an unavailable dataset, and keeps asking', async () => {
    getMock.mockResolvedValue({
      versions: { profile: 'p1' },
      unavailable: ['contacts'],
    })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(refreshed.contacts).toEqual([])

    // Next check: the server has recovered, and the dataset refreshes normally. Nothing about the
    // first answer disabled it.
    getMock.mockResolvedValue({ versions: { profile: 'p1', contacts: 'c9' }, unavailable: [] })
    target.advance(60_000)
    target.tick()
    await flush()

    expect(refreshed.contacts).toEqual(['c9'])
    loop.stop()
  })

  // One store failing must not freeze the other four — the alternative is strictly worse than the
  // problem it would be protecting against.
  it('isolates a failing dataset', async () => {
    getMock.mockResolvedValue(fullResponse)
    useContactsStore().refreshIfVersionDiffers = vi.fn(async () => {
      throw new Error('store bug')
    })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(refreshed.profile).toEqual(['p1'])
    expect(refreshed.scans).toEqual(['s1'])
    expect(refreshed.handouts).toEqual(['h1'])
    expect(refreshed.checkpoints).toEqual(['k1'])

    // And the loop is still alive.
    target.advance(60_000)
    target.tick()
    await flush()
    expect(refreshed.profile).toEqual(['p1', 'p1'])
    loop.stop()
  })

  // A server ahead of an installed client is a normal state for a PWA: the datasets this build knows
  // must keep working rather than the whole check failing on an unknown name.
  it('ignores a dataset it does not know', async () => {
    getMock.mockResolvedValue({ versions: { profile: 'p1', glimt: 'g1' }, unavailable: [] })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(refreshed.profile).toEqual(['p1'])
    loop.stop()
  })

  it('does nothing while nobody is signed in', async () => {
    useSessionStore().user = null
    getMock.mockResolvedValue(fullResponse)
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(getMock).not.toHaveBeenCalled()
    loop.stop()
  })

  // A failed check is a non-event: cached copies stay, and the panes' own staleness affordances say so.
  it('survives a failed check without dispatching anything', async () => {
    getMock.mockRejectedValue(new Error('offline'))
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(refreshed.profile).toEqual([])

    getMock.mockResolvedValue(fullResponse)
    target.advance(60_000)
    target.tick()
    await flush()
    expect(refreshed.profile).toEqual(['p1'])
    loop.stop()
  })

  // A 401 must not be retried on every foreground for the rest of the app's life; the existing auth
  // handling owns the redirect.
  it('stops asking after a 401', async () => {
    getMock.mockRejectedValue(new HttpError(401, 'unauthorized'))
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()
    expect(getMock).toHaveBeenCalledTimes(1)

    target.advance(60_000)
    target.tick()
    await flush()
    target.goOnline()
    await flush()

    expect(getMock).toHaveBeenCalledTimes(1)
    loop.stop()
  })
})

describe('useSyncLoop and the race area', () => {
  // The race area has nothing to refresh, so the only thing a changed version can do is tell the user
  // their downloaded map no longer covers the event (task 294). It must never trigger a download.
  it('reports a stale tile area when the version moved past what was downloaded', async () => {
    rememberTileAreaVersion('r-old')
    getMock.mockResolvedValue({ versions: { race_area: 'r-new' }, unavailable: [] })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(useOfflineStore().statuses.tiles.updateAvailable).toBe(true)
    loop.stop()
  })

  it('reports nothing when the downloaded area is current', async () => {
    rememberTileAreaVersion('r-1')
    getMock.mockResolvedValue({ versions: { race_area: 'r-1' }, unavailable: [] })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(useOfflineStore().statuses.tiles.updateAvailable).toBe(false)
    loop.stop()
  })

  // Nobody who never downloaded a map should be told their map is out of date — including every device
  // that downloaded tiles before this version was recorded at all.
  it('claims nothing when no tiles were ever downloaded', async () => {
    getMock.mockResolvedValue({ versions: { race_area: 'r-new' }, unavailable: [] })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(useOfflineStore().statuses.tiles.updateAvailable).toBe(false)
    loop.stop()
  })
})

describe('useSyncLoop served values', () => {
  // The 02:00 lever: a widened interval has to take effect on a device that may not be reloaded for
  // hours, so the timer is restarted rather than left on the old period.
  it('adopts a changed interval without a reload', async () => {
    getMock.mockResolvedValue({ ...fullResponse, interval_seconds: 60 })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()
    expect(target.intervalMs()).toBe(60_000)

    getMock.mockResolvedValue({ ...fullResponse, interval_seconds: 300 })
    target.advance(60_000)
    target.tick()
    await flush()

    expect(target.intervalMs()).toBe(300_000)
    loop.stop()
  })

  // Zero disables the interval and NOTHING else. This is the distinction an operator would get wrong,
  // so it is the one with its own test.
  it('treats a zero interval as "no timer", not "no checks"', async () => {
    getMock.mockResolvedValue({ ...fullResponse, interval_seconds: 0 })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()

    expect(target.activeTimers()).toBe(0)

    // Foreground still checks.
    target.advance(60_000)
    target.setVisible(false)
    target.setVisible(true)
    await flush()
    expect(getMock).toHaveBeenCalledTimes(2)

    // And so does reconnect.
    target.advance(60_000)
    target.goOnline()
    await flush()
    expect(getMock).toHaveBeenCalledTimes(3)
    loop.stop()
  })

  it('adopts a served debounce', async () => {
    getMock.mockResolvedValue({ ...fullResponse, debounce_seconds: 30 })
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()
    expect(getMock).toHaveBeenCalledTimes(1)

    // Well inside the newly-served 30 s window, and outside the 5 s default — so this asserts the
    // served value was actually adopted rather than the default still being in force.
    target.advance(10_000)
    target.setVisible(false)
    target.setVisible(true)
    await flush()
    expect(getMock).toHaveBeenCalledTimes(1)
    loop.stop()
  })

  // The manual refresh control (task 282) uses this path, so the user's tap is not swallowed by a
  // window they cannot see.
  it('exposes a forced check that ignores the debounce', async () => {
    getMock.mockResolvedValue(fullResponse)
    const target = fakeTarget()
    const loop = useSyncLoop({ target })
    await flush()
    expect(getMock).toHaveBeenCalledTimes(1)

    await loop.check({ force: true })
    await flush()

    expect(getMock).toHaveBeenCalledTimes(2)
    loop.stop()
  })
})
