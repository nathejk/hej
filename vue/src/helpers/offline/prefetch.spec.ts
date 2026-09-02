import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import type { Role } from '@/config/roles'
import type { FreshnessTarget } from '@/composables/useFreshnessLoop'
import { useQuietPrefetch } from '@/helpers/offline/prefetch'
import { useContactsStore } from '@/stores/contacts.store'
import { useSessionStore } from '@/stores/session.store'

// A controllable stand-in for document/window, same seam the freshness loop uses.
function fakeTarget() {
  let visible = true
  const visibility: (() => void)[] = []
  const online: (() => void)[] = []
  const intervals: number[] = []

  return {
    target: {
      isVisible: () => visible,
      onVisibilityChange(handler: () => void) {
        visibility.push(handler)
        return () => {}
      },
      onOnline(handler: () => void) {
        online.push(handler)
        return () => {}
      },
      setInterval(_handler: () => void, ms: number) {
        intervals.push(ms)
        return intervals.length
      },
      clearInterval() {},
    } satisfies FreshnessTarget,
    hide() {
      visible = false
      visibility.forEach((h) => h())
    },
    show() {
      visible = true
      visibility.forEach((h) => h())
    },
    reconnect: () => online.forEach((h) => h()),
    intervals: () => intervals,
  }
}

function signIn(role: Role) {
  useSessionStore().user = { userId: 'u-1', role }
}

// Let the loop's async check settle. Two awaits deep (the loop awaits the check, the check awaits the
// store), so a single microtask tick is not enough — and the loop's overlap guard means a premature
// second trigger would be swallowed rather than counted, which is how this test lies if rushed.
const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('the quiet prefetch', () => {
  it('fetches without being asked, for a role that has the pane', async () => {
    const { target } = fakeTarget()
    signIn('bandit')

    const contacts = useContactsStore()
    let calls = 0
    contacts.refreshIfStale = async () => {
      calls++
      return true
    }

    useQuietPrefetch({ target })
    await Promise.resolve()

    expect(calls).toBe(1)
  })

  // The gate that matters at scale. Without it, every spejder device asks on every foreground for a
  // directory the BFF will always refuse — a few hundred phones generating 403s all race, for a pane
  // they cannot open.
  it('never asks on behalf of a spejder', async () => {
    const { target } = fakeTarget()
    signIn('spejder')

    const contacts = useContactsStore()
    let calls = 0
    contacts.refreshIfStale = async () => {
      calls++
      return true
    }

    useQuietPrefetch({ target })
    await Promise.resolve()

    expect(calls).toBe(0)
  })

  it('does not ask before anyone is signed in', async () => {
    const { target } = fakeTarget()

    const contacts = useContactsStore()
    let calls = 0
    contacts.refreshIfStale = async () => {
      calls++
      return true
    }

    useQuietPrefetch({ target })
    await Promise.resolve()

    expect(calls).toBe(0)
  })

  // The client's idea of a role can be wrong or stale; the BFF is the authority. Once it has said no,
  // we stop asking rather than retrying every time the app comes forward.
  it('stops asking once the server has refused', async () => {
    const { target } = fakeTarget()
    signIn('bandit')

    const contacts = useContactsStore()
    contacts.forbidden = true
    let calls = 0
    contacts.refreshIfStale = async () => {
      calls++
      return true
    }

    useQuietPrefetch({ target })
    await Promise.resolve()

    expect(calls).toBe(0)
  })

  // Details and portraits churn hardest in the run-up, so the catch-up is the point — and this is the
  // device whose owner never opens Kontakter and would otherwise never get one.
  it('catches up on foreground and on reconnect', async () => {
    const t = fakeTarget()
    signIn('crew')

    const contacts = useContactsStore()
    let calls = 0
    contacts.refreshIfStale = async () => {
      calls++
      return true
    }

    useQuietPrefetch({ target: t.target })
    await flush()
    expect(calls).toBe(1)

    t.hide()
    t.show()
    await flush()
    expect(calls).toBe(2)

    t.reconnect()
    await flush()
    expect(calls).toBe(3)
  })

  // No timer, deliberately: a phone in a pocket polling for a directory nobody is reading spends
  // battery and BFF capacity for nothing. The during-race interval belongs to the pane, where
  // somebody is actually looking at the answer.
  it('starts no interval, so it adds no continuous traffic', async () => {
    const t = fakeTarget()
    signIn('bandit')
    useContactsStore().refreshIfStale = async () => true

    useQuietPrefetch({ target: t.target })
    await Promise.resolve()

    expect(t.intervals()).toEqual([])
  })
})
