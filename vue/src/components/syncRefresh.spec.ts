import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { fetchWrapper } from '@/helpers'
import { refreshNow, useSyncLoop } from '@/composables/useSyncLoop'
import type { FreshnessTarget } from '@/composables/useFreshnessLoop'
import { useProfileStore } from '@/stores/profile.store'
import { useSessionStore } from '@/stores/session.store'

// What the manual refresh control depends on (task 282). The button's own rendering is thin — an icon,
// a disabled flag and a sentence — so what is worth testing is the seam underneath it: that a tap
// reaches the *app's* loop, forced, and comes back with an outcome honest enough to show a user.

function stillTarget(): FreshnessTarget {
  return {
    isVisible: () => true,
    onVisibilityChange: () => () => {},
    onOnline: () => () => {},
    setInterval: () => 1,
    clearInterval: () => {},
    now: () => 0,
  }
}

let getMock: ReturnType<typeof vi.fn>

// Lets the loop's mount check settle. Without this, a `refreshNow()` in the same turn is dropped by the
// overlap guard — correct behaviour (and its own test below), but not what most of these are about.
async function flush() {
  for (let i = 0; i < 6; i++) await Promise.resolve()
}

beforeEach(() => {
  setActivePinia(createPinia())
  getMock = vi.fn()
  fetchWrapper.get = getMock as never
  useSessionStore().user = { userId: 'u-1', role: 'spejder' } as never
  useProfileStore().refreshIfVersionDiffers = vi.fn(async () => true)
})

describe('refreshNow', () => {
  // No loop running means no app: answering "skipped" is honest, and starting one on a button press
  // would create a loop outside the app's lifecycle that nothing ever stops.
  it('reports skipped when no loop is running', async () => {
    expect(await refreshNow()).toEqual({ status: 'skipped', refreshed: [] })
  })

  it('reports which datasets were refreshed', async () => {
    getMock.mockResolvedValue({ versions: { profile: 'p1' }, unavailable: [] })
    const loop = useSyncLoop({ target: stillTarget(), debounceSeconds: 300 })
    await flush()

    const outcome = await refreshNow()

    expect(outcome.status).toBe('refreshed')
    expect(outcome.refreshed).toEqual(['profile'])
    loop.stop()
  })

  // The common case, and the one the control must still speak up about: a silent control reads as
  // broken and gets tapped again.
  it('reports unchanged when everything is current', async () => {
    getMock.mockResolvedValue({ versions: { profile: 'p1' }, unavailable: [] })
    useProfileStore().refreshIfVersionDiffers = vi.fn(async () => false)
    const loop = useSyncLoop({ target: stillTarget(), debounceSeconds: 300 })
    await flush()

    expect((await refreshNow()).status).toBe('unchanged')
    loop.stop()
  })

  // A long served debounce must not swallow a tap the user can see they made.
  it('ignores the debounce', async () => {
    getMock.mockResolvedValue({ versions: { profile: 'p1' }, unavailable: [], debounce_seconds: 300 })
    const loop = useSyncLoop({ target: stillTarget(), debounceSeconds: 300 })
    await flush()
    const before = getMock.mock.calls.length

    await refreshNow()

    expect(getMock.mock.calls.length).toBe(before + 1)
    loop.stop()
  })

  // "No signal" and "the server said no" are different sentences to a user, and only one of them is
  // advice they can act on.
  it('distinguishes offline from a server error', async () => {
    getMock.mockRejectedValue(new Error('network down'))
    const loop = useSyncLoop({ target: stillTarget(), debounceSeconds: 300 })
    await flush()

    expect((await refreshNow()).status).toBe('offline')

    const { HttpError } = await import('@/helpers')
    getMock.mockRejectedValue(new HttpError(500, 'boom'))
    expect((await refreshNow()).status).toBe('error')
    loop.stop()
  })

  // Telling somebody "everything is up to date" on the strength of a check that never ran is exactly
  // the confident wrong answer this control exists to remove.
  it('does not borrow the previous answer when a check is already in flight', async () => {
    let release: ((v: unknown) => void) | null = null
    const releasers: ((v: unknown) => void)[] = []
    getMock.mockImplementation(
      () => new Promise((resolve) => releasers.push(resolve)),
    )
    const loop = useSyncLoop({ target: stillTarget(), debounceSeconds: 0 })

    // The mount check is in flight; a tap now is dropped by the overlap guard.
    const outcome = await refreshNow()
    expect(outcome.status).toBe('skipped')

    release = releasers.shift() ?? null
    release?.({ versions: {}, unavailable: [] })
    loop.stop()
  })

  it('stops reporting once the session is gone', async () => {
    const { HttpError } = await import('@/helpers')
    getMock.mockRejectedValue(new HttpError(401, 'unauthorized'))
    const loop = useSyncLoop({ target: stillTarget(), debounceSeconds: 0 })
    await flush()

    expect((await refreshNow()).status).toBe('unauthenticated')

    // And a second tap does not put the request back on the wire — nor does it claim to be "already
    // refreshing", which is what a naive skip would have said to somebody who has been logged out.
    const before = getMock.mock.calls.length
    expect((await refreshNow()).status).toBe('unauthenticated')
    expect(getMock.mock.calls.length).toBe(before)
    loop.stop()
  })
})
