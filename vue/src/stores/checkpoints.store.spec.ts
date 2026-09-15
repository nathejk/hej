import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useCheckpointsStore, type Checkpoint } from '@/stores/checkpoints.store'
import { useSessionStore } from '@/stores/session.store'
import type { ScopedStorage } from '@/helpers/profileStorage'

let getMock: (url: string) => Promise<unknown>

vi.mock('@/helpers', () => ({
  fetchWrapper: { get: (url: string) => getMock(url) },
}))

// A fake Storage, per the seam the contacts store documents: these modules take their browser environment
// as an argument rather than reading globals, so the specs can run in node.
function fakeStorage(): ScopedStorage & { data: Map<string, string> } {
  const data = new Map<string, string>()
  return {
    data,
    getItem: (k) => data.get(k) ?? null,
    setItem: (k, v) => void data.set(k, v),
    removeItem: (k) => void data.delete(k),
  }
}

function apiCheckpoint(over: Partial<Record<string, unknown>> = {}) {
  return {
    id: 'cp-1',
    name: 'Post 1',
    checkgroup: 'cg-1',
    sort_order: 0,
    lat: 56.1382,
    lng: 9.5521,
    open_from: 1750000000,
    open_until: 1750003600,
    open_duration_minutes: 0,
    ...over,
  }
}

function signIn(userId = 'user-1') {
  const session = useSessionStore()
  session.user = { userId, name: 'Signe', role: 'spejder' } as never
}

describe('checkpoints.store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getMock = () => Promise.resolve({ checkpoints: [] })
  })

  it('maps the snake_case payload at the store boundary', async () => {
    getMock = () => Promise.resolve({ checkpoints: [apiCheckpoint()] })
    const store = useCheckpointsStore()

    await store.fetch()

    expect(store.checkpoints).toHaveLength(1)
    const cp = store.checkpoints[0] as Checkpoint
    expect(cp.sortOrder).toBe(0)
    expect(cp.openFrom).toBe(1750000000)
    expect(cp.openDurationMinutes).toBe(0)
    expect(cp.lat).toBeCloseTo(56.1382)
  })

  // An empty list is a normal answer — a patrol before its first handout, and every personnel user without
  // a patrol. It must be a loaded state, not an error, or the map would show a failure notice all evening.
  it('treats an empty list as loaded rather than as an error', async () => {
    getMock = () => Promise.resolve({ checkpoints: [] })
    const store = useCheckpointsStore()

    await store.fetch()

    expect(store.checkpoints).toEqual([])
    expect(store.loaded).toBe(true)
    expect(store.error).toBe('')
    expect(store.hasAny).toBe(false)
  })

  it('tolerates a null list', async () => {
    getMock = () => Promise.resolve({ checkpoints: null })
    const store = useCheckpointsStore()

    await store.fetch()

    expect(store.checkpoints).toEqual([])
  })

  // Replace, never merge (PRD 009 §6). A post the server stops sending has been withdrawn, and nothing will
  // ever tell the client so explicitly — a merge would leave it on the map for the rest of the event.
  it('replaces the list rather than merging it', async () => {
    getMock = () => Promise.resolve({ checkpoints: [apiCheckpoint({ id: 'cp-1' }), apiCheckpoint({ id: 'cp-2' })] })
    const store = useCheckpointsStore()
    await store.fetch()
    expect(store.checkpoints).toHaveLength(2)

    getMock = () => Promise.resolve({ checkpoints: [apiCheckpoint({ id: 'cp-1' })] })
    await store.fetch()

    expect(store.checkpoints.map((c) => c.id)).toEqual(['cp-1'])
  })

  // The map is the page people open when something has gone wrong. A failed refresh keeps what is held and
  // says so; it must never throw, and must never blank the map.
  it('keeps the cached copy when a refresh fails', async () => {
    getMock = () => Promise.resolve({ checkpoints: [apiCheckpoint()] })
    const store = useCheckpointsStore()
    await store.fetch()

    getMock = () => Promise.reject(new Error('offline'))
    await expect(store.fetch()).resolves.toBeUndefined()

    expect(store.checkpoints).toHaveLength(1)
    expect(store.error).not.toBe('')
    expect(store.loading).toBe(false)
  })

  it('persists per profile and hydrates from the cache', async () => {
    const storage = fakeStorage()
    signIn('user-1')
    getMock = () => Promise.resolve({ checkpoints: [apiCheckpoint()] })

    const store = useCheckpointsStore()
    store.storage = storage
    await store.fetch()

    // A fresh store on the same profile finds the copy without asking the network.
    setActivePinia(createPinia())
    signIn('user-1')
    const reopened = useCheckpointsStore()
    reopened.storage = storage
    reopened.hydrate()

    expect(reopened.checkpoints).toHaveLength(1)
    expect(reopened.loaded).toBe(true)
    expect(reopened.syncedAt).toBeGreaterThan(0)
  })

  // Several profiles share one phone. A sibling switching in must not inherit the previous patrol's map:
  // the posts they may see are not the same posts.
  it('does not leak one profile’s map to another', async () => {
    const storage = fakeStorage()
    signIn('user-1')
    getMock = () => Promise.resolve({ checkpoints: [apiCheckpoint()] })

    const first = useCheckpointsStore()
    first.storage = storage
    await first.fetch()

    setActivePinia(createPinia())
    signIn('user-2')
    const second = useCheckpointsStore()
    second.storage = storage
    second.hydrate()

    expect(second.checkpoints).toEqual([])
  })

  // Signed out there is no key, and no device-wide fallback on purpose: writing one would put this
  // profile's map under a key the next profile reads.
  it('does not touch storage without a signed-in profile', async () => {
    const storage = fakeStorage()
    getMock = () => Promise.resolve({ checkpoints: [apiCheckpoint()] })

    const store = useCheckpointsStore()
    store.storage = storage
    await store.fetch()

    expect(storage.data.size).toBe(0)
    // The data is still usable in memory for this session.
    expect(store.checkpoints).toHaveLength(1)
  })

  // What comes out of storage is input, not state: a value from a schema this build does not know is
  // discarded rather than trusted.
  it('discards a stored payload from an unknown schema', () => {
    const storage = fakeStorage()
    signIn('user-1')
    storage.setItem(
      'hej.checkpoints.v1.user-1',
      JSON.stringify({ schema: 99, syncedAt: 1, checkpoints: [apiCheckpoint()] }),
    )

    const store = useCheckpointsStore()
    store.storage = storage
    store.hydrate()

    expect(store.checkpoints).toEqual([])
    expect(store.loaded).toBe(false)
  })

  it('survives corrupt stored JSON', () => {
    const storage = fakeStorage()
    signIn('user-1')
    storage.setItem('hej.checkpoints.v1.user-1', '{not json')

    const store = useCheckpointsStore()
    store.storage = storage

    expect(() => store.hydrate()).not.toThrow()
    expect(store.checkpoints).toEqual([])
  })

  // A quota-exceeded write throws on Safari. The map works from memory regardless, and white-screening the
  // one page people open when lost would be the worse outcome.
  it('survives a storage write that throws', async () => {
    signIn('user-1')
    getMock = () => Promise.resolve({ checkpoints: [apiCheckpoint()] })

    const store = useCheckpointsStore()
    store.storage = {
      getItem: () => null,
      setItem: () => {
        throw new Error('QuotaExceededError')
      },
      removeItem: () => {},
    }

    await expect(store.fetch()).resolves.toBeUndefined()
    expect(store.checkpoints).toHaveLength(1)
  })

  it('indexes checkpoints by id for joining a scan to its post', async () => {
    getMock = () =>
      Promise.resolve({ checkpoints: [apiCheckpoint({ id: 'cp-1', name: 'Post 1' })] })
    const store = useCheckpointsStore()
    await store.fetch()

    expect(store.byId.get('cp-1')?.name).toBe('Post 1')
    expect(store.byId.get('cp-nope')).toBeUndefined()
  })

  // Route order is (checkgroup order, checkpoint order) and only the BFF has both halves, so the server
  // sends the list already ordered and the store keeps it. A client-side re-sort would need the group order
  // shipped too, and one that got it wrong would point arrows at the wrong "next" post while looking fine.
  it('preserves the order the BFF sent', async () => {
    getMock = () =>
      Promise.resolve({
        checkpoints: [
          apiCheckpoint({ id: 'cp-early', checkgroup: 'cg-1', sort_order: 5 }),
          apiCheckpoint({ id: 'cp-late', checkgroup: 'cg-2', sort_order: 0 }),
        ],
      })
    const store = useCheckpointsStore()

    await store.fetch()

    expect(store.checkpoints.map((c) => c.id)).toEqual(['cp-early', 'cp-late'])
  })
})
