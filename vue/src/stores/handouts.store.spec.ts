import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useHandoutsStore } from '@/stores/handouts.store'
import { useSessionStore } from '@/stores/session.store'
import type { ScopedStorage } from '@/helpers/profileStorage'

let getMock: (url: string) => Promise<unknown>

vi.mock('@/helpers', () => ({
  fetchWrapper: { get: (url: string) => getMock(url) },
}))

// A fake Storage, per the seam the stores document: these modules take their browser environment as an
// argument rather than reading globals, so the specs can run in node.
function fakeStorage(): ScopedStorage & { data: Map<string, string> } {
  const data = new Map<string, string>()
  return {
    data,
    getItem: (k) => data.get(k) ?? null,
    setItem: (k, v) => void data.set(k, v),
    removeItem: (k) => void data.delete(k),
  }
}

function apiHandout(over: Partial<Record<string, unknown>> = {}) {
  return {
    name: 'Etape 1',
    format: 'a4',
    qr_id: '1042',
    handed_out: '2026-08-24T18:40:00Z',
    still_held: true,
    ...over,
  }
}

function signIn(userId = 'user-1') {
  const session = useSessionStore()
  session.user = { userId, name: 'Signe', role: 'spejder' } as never
}

describe('handouts.store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getMock = () => Promise.resolve({ handouts: [] })
  })

  it('maps the snake_case payload at the store boundary', async () => {
    getMock = () => Promise.resolve({ handouts: [apiHandout()] })
    const store = useHandoutsStore()

    await store.fetch()

    expect(store.handouts).toHaveLength(1)
    const h = store.handouts[0]!
    expect(h.name).toBe('Etape 1')
    expect(h.qrId).toBe('1042')
    expect(h.stillHeld).toBe(true)
    expect(h.handedOut.getTime()).toBe(Date.parse('2026-08-24T18:40:00Z'))
  })

  // The privacy rule at the data layer: a reassigned sheet arrives with still_held=false and nothing about
  // where it went. The mapped Handout must carry no field that could name another team.
  it('carries no field that could name another team', async () => {
    getMock = () => Promise.resolve({ handouts: [apiHandout({ still_held: false })] })
    const store = useHandoutsStore()

    await store.fetch()

    const keys = Object.keys(store.handouts[0]!)
    for (const key of keys) {
      const lower = key.toLowerCase()
      for (const banned of ['team', 'successor', 'moved', 'flyttet', 'hold']) {
        expect(lower).not.toContain(banned)
      }
    }
  })

  it('treats an empty list as loaded rather than as an error', async () => {
    getMock = () => Promise.resolve({ handouts: [] })
    const store = useHandoutsStore()

    await store.fetch()

    expect(store.handouts).toEqual([])
    expect(store.loaded).toBe(true)
    expect(store.error).toBe('')
    expect(store.hasAny).toBe(false)
  })

  it('tolerates a null list', async () => {
    getMock = () => Promise.resolve({ handouts: null })
    const store = useHandoutsStore()

    await store.fetch()

    expect(store.handouts).toEqual([])
  })

  // Replace, never merge (PRD 009 §6). A sheet the server stops sending has been reassigned; a merge would
  // leave it listed as held for the rest of the event.
  it('replaces the list rather than merging it', async () => {
    getMock = () =>
      Promise.resolve({ handouts: [apiHandout({ qr_id: '1' }), apiHandout({ qr_id: '2' })] })
    const store = useHandoutsStore()
    await store.fetch()
    expect(store.handouts).toHaveLength(2)

    getMock = () => Promise.resolve({ handouts: [apiHandout({ qr_id: '1' })] })
    await store.fetch()

    expect(store.handouts.map((h) => h.qrId)).toEqual(['1'])
  })

  it('keeps the cached copy when a refresh fails', async () => {
    getMock = () => Promise.resolve({ handouts: [apiHandout()] })
    const store = useHandoutsStore()
    await store.fetch()

    getMock = () => Promise.reject(new Error('offline'))
    // Reports failure rather than throwing: the drawer keeps its sheets, and a versioned caller needs
    // to know not to record the version it fetched against (task 287).
    await expect(store.fetch()).resolves.toBe(false)

    expect(store.handouts).toHaveLength(1)
    expect(store.error).not.toBe('')
    expect(store.loading).toBe(false)
  })

  it('persists per profile and hydrates from the cache', async () => {
    const storage = fakeStorage()
    signIn('user-1')
    getMock = () => Promise.resolve({ handouts: [apiHandout()] })

    const store = useHandoutsStore()
    store.storage = storage
    await store.fetch()

    // A fresh store on the same profile finds the copy without asking the network, dates intact.
    setActivePinia(createPinia())
    signIn('user-1')
    const reopened = useHandoutsStore()
    reopened.storage = storage
    reopened.hydrate()

    expect(reopened.handouts).toHaveLength(1)
    expect(reopened.handouts[0]!.handedOut.getTime()).toBe(Date.parse('2026-08-24T18:40:00Z'))
    expect(reopened.loaded).toBe(true)
    expect(reopened.syncedAt).toBeGreaterThan(0)
  })

  // Several profiles share one phone. A sibling switching in must not inherit the previous patrol's sheets.
  it('does not leak one profile’s sheets to another', async () => {
    const storage = fakeStorage()
    signIn('user-1')
    getMock = () => Promise.resolve({ handouts: [apiHandout()] })

    const first = useHandoutsStore()
    first.storage = storage
    await first.fetch()

    setActivePinia(createPinia())
    signIn('user-2')
    const second = useHandoutsStore()
    second.storage = storage
    second.hydrate()

    expect(second.handouts).toEqual([])
  })

  it('does not touch storage without a signed-in profile', async () => {
    const storage = fakeStorage()
    getMock = () => Promise.resolve({ handouts: [apiHandout()] })

    const store = useHandoutsStore()
    store.storage = storage
    await store.fetch()

    expect(storage.data.size).toBe(0)
    expect(store.handouts).toHaveLength(1)
  })

  it('discards a stored payload from an unknown schema', () => {
    const storage = fakeStorage()
    signIn('user-1')
    storage.setItem(
      'hej.handouts.v1.user-1',
      JSON.stringify({ schema: 99, syncedAt: 1, handouts: [] }),
    )

    const store = useHandoutsStore()
    store.storage = storage
    store.hydrate()

    expect(store.handouts).toEqual([])
    expect(store.loaded).toBe(false)
  })

  it('survives corrupt stored JSON', () => {
    const storage = fakeStorage()
    signIn('user-1')
    storage.setItem('hej.handouts.v1.user-1', '{not json')

    const store = useHandoutsStore()
    store.storage = storage

    expect(() => store.hydrate()).not.toThrow()
    expect(store.handouts).toEqual([])
  })
})
