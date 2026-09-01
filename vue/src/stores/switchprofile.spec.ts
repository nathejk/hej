import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/helpers', async () => {
  const actual = await vi.importActual<typeof import('@/helpers')>('@/helpers')
  return {
    ...actual,
    fetchWrapper: {
      get: (url: string) => getMock(url),
      post: (url: string, body?: unknown) => postMock(url, body),
      put: vi.fn(),
      delete: vi.fn(),
    },
  }
})

import { NetworkError } from '@/helpers'
import { useSessionStore } from '@/stores/session.store'

let getMock: (url: string) => Promise<unknown>
let postMock: (url: string, body?: unknown) => Promise<unknown>

// Switching profile, client side (PRD 012, task 182).
//
// The switch is deliberately SMS-free — the caller proved control of the number at login — so what
// matters here is that the control is offered only when there is something to switch to, and that a
// failure leaves the user signed in as they were rather than half-switched.

beforeEach(() => {
  setActivePinia(createPinia())
  getMock = () => Promise.reject(new Error('unexpected GET'))
  postMock = () => Promise.reject(new Error('unexpected POST'))
})

describe('canSwitchProfile', () => {
  it('is false before /api/me has answered', () => {
    // Notably also the offline-start case: the count is not persisted with the remembered identity,
    // because a switch cannot complete without the network anyway.
    expect(useSessionStore().canSwitchProfile).toBe(false)
  })

  it('is false for a number with one profile', async () => {
    getMock = () => Promise.resolve({ user_id: 'u1', role: 'bandit', profile_count: 1 })

    const session = useSessionStore()
    await session.fetchMe()

    expect(session.profileCount).toBe(1)
    expect(session.canSwitchProfile).toBe(false)
  })

  it('is true for a number with several profiles', async () => {
    getMock = () => Promise.resolve({ user_id: 'u1', role: 'bandit', profile_count: 3 })

    const session = useSessionStore()
    await session.fetchMe()

    expect(session.canSwitchProfile).toBe(true)
  })

  // An older BFF, or one that could not reach the directory, sends nothing. Hiding the control is the
  // safe direction: offering it is what would mislead.
  it('is false when the count is absent', async () => {
    getMock = () => Promise.resolve({ user_id: 'u1', role: 'bandit' })

    const session = useSessionStore()
    await session.fetchMe()

    expect(session.profileCount).toBe(0)
    expect(session.canSwitchProfile).toBe(false)
  })

  it('is cleared on sign-out', async () => {
    getMock = () => Promise.resolve({ user_id: 'u1', role: 'bandit', profile_count: 2 })
    postMock = () => Promise.resolve({})

    const session = useSessionStore()
    await session.fetchMe()
    expect(session.canSwitchProfile).toBe(true)

    await session.logout()
    expect(session.canSwitchProfile).toBe(false)
  })

  // The chooser at login knows the count already — it is the length of the candidate list — so the
  // switcher must not be missing for the rest of the session that just used it.
  it('is set by completing the login chooser, without another /api/me', async () => {
    const session = useSessionStore()

    postMock = (url) => {
      if (url === '/api/auth/verify') {
        return Promise.resolve({
          choice_token: 'tok',
          candidates: [
            { user_id: 'a', name: 'Freja', team: 'Patrulje Ravnene' },
            { user_id: 'b', name: 'Villads', team: 'Patrulje Ravnene' },
          ],
        })
      }
      return Promise.resolve({ user_id: 'a', role: 'spejder' })
    }

    await session.verify('30000008', '1234')
    expect(session.needsChoice).toBe(true)

    await session.choose('a')

    expect(session.profileCount).toBe(2)
    expect(session.canSwitchProfile).toBe(true)
  })
})

describe('startProfileSwitch', () => {
  it('stores the token and candidates for choose() to complete', async () => {
    const session = useSessionStore()
    postMock = () =>
      Promise.resolve({
        choice_token: 'switch-token',
        candidates: [
          { user_id: 'a', name: 'Freja', team: 'Patrulje Ravnene' },
          { user_id: 'b', name: 'Freja', team: 'Patrulje Ulvene' },
        ],
      })

    const candidates = await session.startProfileSwitch()

    expect(candidates).toHaveLength(2)
    expect(session.needsChoice).toBe(true)
    // Same state the login chooser uses, so one code path completes either.
    expect(session.choiceCandidates[0].team).toBe('Patrulje Ravnene')
  })

  it('hits the switch endpoint, not the login one', async () => {
    const seen: string[] = []
    postMock = (url) => {
      seen.push(url)
      return Promise.resolve({ choice_token: 't', candidates: [] })
    }

    await useSessionStore().startProfileSwitch()
    expect(seen).toEqual(['/api/auth/switch'])
  })

  // Unlike fetchMe, this throws: the caller has an explicit user action to report on, and silently
  // doing nothing after a tap is worse than a message.
  it('throws offline and leaves the session untouched', async () => {
    getMock = () => Promise.resolve({ user_id: 'u1', role: 'bandit', profile_count: 2 })
    const session = useSessionStore()
    await session.fetchMe()

    postMock = () => Promise.reject(new NetworkError('/api/auth/switch'))

    await expect(session.startProfileSwitch()).rejects.toBeInstanceOf(NetworkError)

    // Still signed in as the same person, with no pending choice to confuse the UI.
    expect(session.user?.userId).toBe('u1')
    expect(session.needsChoice).toBe(false)
  })

  it('leaves the session untouched when the switch is refused', async () => {
    getMock = () => Promise.resolve({ user_id: 'u1', role: 'bandit', profile_count: 2 })
    const session = useSessionStore()
    await session.fetchMe()

    postMock = () => Promise.reject(new Error('409'))

    await expect(session.startProfileSwitch()).rejects.toThrow()
    expect(session.user?.userId).toBe('u1')
  })
})

describe('completing a switch', () => {
  it('signs in as the chosen profile', async () => {
    getMock = () => Promise.resolve({ user_id: 'u1', role: 'bandit', profile_count: 2 })
    const session = useSessionStore()
    await session.fetchMe()

    postMock = (url) => {
      if (url === '/api/auth/switch') {
        return Promise.resolve({
          choice_token: 'tok',
          candidates: [
            { user_id: 'u1', name: 'Freja' },
            { user_id: 'u2', name: 'Freja' },
          ],
        })
      }
      return Promise.resolve({ user_id: 'u2', role: 'samarit' })
    }

    await session.startProfileSwitch()
    const identity = await session.choose('u2')

    expect(identity.userId).toBe('u2')
    // The role follows the profile: a duplicate registration may be a spejder on one row and crew
    // on another, which is why the pane and nav have to be rebuilt after a switch.
    expect(session.role).toBe('samarit')
    expect(session.needsChoice).toBe(false)
  })

  it('keeps the previous profile signed in when choosing fails', async () => {
    getMock = () => Promise.resolve({ user_id: 'u1', role: 'bandit', profile_count: 2 })
    const session = useSessionStore()
    await session.fetchMe()

    postMock = (url) => {
      if (url === '/api/auth/switch') {
        return Promise.resolve({ choice_token: 'tok', candidates: [{ user_id: 'u2', name: 'X' }] })
      }
      return Promise.reject(new Error('401'))
    }

    await session.startProfileSwitch()
    await expect(session.choose('u2')).rejects.toThrow()

    expect(session.user?.userId).toBe('u1')
    expect(session.role).toBe('bandit')
  })
})
