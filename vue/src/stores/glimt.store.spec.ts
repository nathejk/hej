import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

// Mocked at the module boundary so the store is tested without a network or a DOM fetch.
vi.mock('@/helpers', async () => {
  const actual = await vi.importActual<typeof import('@/helpers')>('@/helpers')
  return {
    ...actual,
    fetchWrapper: {
      get: (url: string) => getMock(url),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn(),
    },
  }
})

import { HttpError } from '@/helpers'
import { glimtMediaUrl, useGlimtStore, type GlimtStorage } from '@/stores/glimt.store'
import { useSessionStore } from '@/stores/session.store'

let getMock: (url: string) => Promise<unknown>

const USER_ID = 'p-viewer'
const STORAGE_KEY = `hej.glimt.v1.${USER_ID}`

function fakeStorage(
  initial: Record<string, string> = {},
): GlimtStorage & { data: Record<string, string> } {
  const data = { ...initial }
  return {
    data,
    getItem: (k) => (k in data ? data[k] : null),
    setItem: (k, v) => {
      data[k] = v
    },
    removeItem: (k) => {
      delete data[k]
    },
  }
}

function storeWith(initial: Record<string, string> = {}) {
  const storage = fakeStorage(initial)
  useSessionStore().user = { userId: USER_ID, role: 'spejder' }
  const store = useGlimtStore()
  store.storage = storage
  return { store, storage }
}

// One glimt as the BFF sends it: snake_case, no author, no blob refs.
function apiGlimt(overrides: Record<string, unknown> = {}) {
  return {
    id: 'g-1',
    hold: { number: '42', name: 'Ørnene', group: 'spejder' },
    own: false,
    audience: 'group',
    caption: 'ved posten',
    created_at: '2026-09-17T21:00:00Z',
    media: [
      { ordinal: 0, kind: 'image', width: 1600, height: 1200, has_thumb: true },
      { ordinal: 1, kind: 'video', width: 1920, height: 1080, duration_ms: 12000, has_thumb: false },
    ],
    hidden: false,
    ...overrides,
  }
}

function storedPayload(over: Record<string, unknown> = {}) {
  return JSON.stringify({
    schema: 1,
    version: 'v1',
    syncedAt: 1_700_000_000_000,
    expiresAt: 0,
    glimt: [
      {
        id: 'g-stored',
        hold: { number: '42', name: 'Ørnene', group: 'spejder' },
        own: false,
        audience: 'group',
        caption: 'fra cachen',
        createdAt: 1_700_000_000_000,
        media: [],
        hidden: false,
      },
    ],
    ...over,
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  getMock = async () => ({ glimt: [] })
})

describe('fetch', () => {
  it('maps the payload to camelCase and persists it', async () => {
    getMock = async () => ({ glimt: [apiGlimt()], expires_at: 9_000_000_000_000 })
    const { store, storage } = storeWith()

    expect(await store.fetch()).toBe(true)
    expect(store.glimt).toHaveLength(1)

    const g = store.glimt[0]
    expect(g.id).toBe('g-1')
    expect(g.hold).toEqual({ number: '42', name: 'Ørnene', group: 'spejder' })
    expect(g.caption).toBe('ved posten')
    expect(g.createdAt).toBe(Date.parse('2026-09-17T21:00:00Z'))
    expect(g.media[1]).toEqual({
      ordinal: 1,
      kind: 'video',
      width: 1920,
      height: 1080,
      durationMs: 12000,
      hasThumb: false,
    })

    expect(store.expiresAt).toBe(9_000_000_000_000)
    expect(storage.data[STORAGE_KEY]).toBeDefined()
  })

  // Replace, never merge. A deleted, hidden or purged glimt must stop existing on a device that
  // already synced it — merging would keep it forever and make every takedown decorative.
  it('replaces the copy rather than merging', async () => {
    const { store } = storeWith({ [STORAGE_KEY]: storedPayload() })
    store.hydrate()
    expect(store.glimt.map((g) => g.id)).toEqual(['g-stored'])

    getMock = async () => ({ glimt: [apiGlimt({ id: 'g-new' })] })
    await store.fetch()

    expect(store.glimt.map((g) => g.id)).toEqual(['g-new'])
  })

  it('empties the feed when the server returns nothing', async () => {
    const { store } = storeWith({ [STORAGE_KEY]: storedPayload() })
    store.hydrate()

    getMock = async () => ({ glimt: null })
    await store.fetch()

    expect(store.glimt).toEqual([])
  })

  // An unrecognised audience falls back to the *narrowest*, so a value this bundle does not
  // understand cannot be rendered as more widely shared than it is.
  it('falls back to the narrowest audience for an unknown value', async () => {
    getMock = async () => ({ glimt: [apiGlimt({ audience: 'everyone' })] })
    const { store } = storeWith()
    await store.fetch()
    expect(store.glimt[0].audience).toBe('group')
  })

  it('treats 403 as "no feed for you" rather than an error', async () => {
    getMock = async () => {
      throw new HttpError(403, 'forbidden')
    }
    const { store, storage } = storeWith({ [STORAGE_KEY]: storedPayload() })
    store.hydrate()

    expect(await store.fetch()).toBe(false)
    expect(store.forbidden).toBe(true)
    expect(store.error).toBe('')
    expect(store.glimt).toEqual([])
    // Cleared, because a role can change mid-event and anything held is now out of scope.
    expect(storage.data[STORAGE_KEY]).toBeUndefined()
  })

  // The BFF distinguishes "unavailable" from "empty" on purpose (task 304), and the client must keep
  // that distinction: an empty feed is a legitimate state a device would cache.
  it('keeps the cached copy when the feature is unavailable', async () => {
    const { store } = storeWith({ [STORAGE_KEY]: storedPayload() })
    store.hydrate()

    getMock = async () => {
      throw new HttpError(503, 'unavailable')
    }
    expect(await store.fetch()).toBe(false)
    expect(store.glimt.map((g) => g.id)).toEqual(['g-stored'])
    expect(store.error).toContain('ikke tilgængelige')
  })

  it('never throws on a failed refresh and keeps what it holds', async () => {
    const { store } = storeWith({ [STORAGE_KEY]: storedPayload() })
    store.hydrate()

    getMock = async () => {
      throw new Error('offline')
    }
    await expect(store.fetch()).resolves.toBe(false)
    expect(store.glimt.map((g) => g.id)).toEqual(['g-stored'])
    expect(store.error).not.toBe('')
  })
})

describe('hydrate', () => {
  it('reads a stored copy once', () => {
    const { store } = storeWith({ [STORAGE_KEY]: storedPayload() })
    store.hydrate()
    expect(store.glimt).toHaveLength(1)
    expect(store.version).toBe('v1')
    expect(store.hydrated).toBe(true)
  })

  // The dormant-device rule, and the only lever we hold over a cache full of photographs of
  // children: a phone that never reopens the app after the event runs no purge, no service worker
  // and no push ever again.
  it('discards a copy past its server-issued deadline and flags it', () => {
    const { store, storage } = storeWith({
      [STORAGE_KEY]: storedPayload({ expiresAt: Date.now() - 1000 }),
    })
    store.hydrate()

    expect(store.glimt).toEqual([])
    expect(store.expired).toBe(true)
    expect(storage.data[STORAGE_KEY]).toBeUndefined()
  })

  it('keeps a copy inside its deadline', () => {
    const { store } = storeWith({
      [STORAGE_KEY]: storedPayload({ expiresAt: Date.now() + 60_000 }),
    })
    store.hydrate()
    expect(store.glimt).toHaveLength(1)
    expect(store.expired).toBe(false)
  })

  // What comes out of storage is input, not state: a half-written value from a killed tab must not
  // reach the render path as `undefined.media`.
  it('rejects malformed stored values without throwing', () => {
    for (const raw of [
      'not json',
      '{}',
      '{"schema":1}',
      JSON.stringify({ schema: 1, version: 'v', syncedAt: 0, expiresAt: 0, glimt: [{ id: 1 }] }),
      JSON.stringify({ schema: 1, version: 'v', syncedAt: 0, expiresAt: 0, glimt: 'nope' }),
    ]) {
      setActivePinia(createPinia())
      const { store } = storeWith({ [STORAGE_KEY]: raw })
      expect(() => store.hydrate()).not.toThrow()
      expect(store.glimt).toEqual([])
    }
  })

  it('discards a payload from a different schema', () => {
    const { store } = storeWith({ [STORAGE_KEY]: storedPayload({ schema: 99 }) })
    store.hydrate()
    expect(store.glimt).toEqual([])
  })

  // No device-wide fallback: writing one would put this profile's feed under a key the next profile
  // reads, and a bandit must not find the crew's group-scoped photographs on a shared phone.
  it('touches no storage when nobody is signed in', () => {
    const storage = fakeStorage({ 'hej.glimt.v1': storedPayload() })
    const store = useGlimtStore()
    store.storage = storage

    expect(store.storageKey).toBeNull()
    store.hydrate()
    expect(store.glimt).toEqual([])
  })

  it('survives storage being unavailable entirely', () => {
    useSessionStore().user = { userId: USER_ID, role: 'spejder' }
    const store = useGlimtStore()
    store.storage = null
    expect(() => store.hydrate()).not.toThrow()
    expect(store.glimt).toEqual([])
  })
})

describe('refreshIfVersionDiffers', () => {
  it('does nothing when the version matches', async () => {
    let calls = 0
    getMock = async () => {
      calls++
      return { glimt: [apiGlimt()] }
    }
    const { store } = storeWith()
    await store.fetch()
    store.version = 'v7'
    const before = store.syncedAt

    expect(await store.refreshIfVersionDiffers('v7')).toBe(false)
    expect(calls).toBe(1)
    // "We checked" and "we refetched" are different facts; the UI shows the second.
    expect(store.syncedAt).toBe(before)
  })

  it('fetches when the version differs and records the new one', async () => {
    getMock = async () => ({ glimt: [apiGlimt()] })
    const { store } = storeWith()

    expect(await store.refreshIfVersionDiffers('v2')).toBe(true)
    expect(store.version).toBe('v2')
  })

  it('fetches when nothing is held, whatever the version', async () => {
    getMock = async () => ({ glimt: [apiGlimt()] })
    const { store } = storeWith()
    expect(store.version).toBe('')
    expect(await store.refreshIfVersionDiffers('')).toBe(true)
  })

  // The subtle rule syncVersions.ts exists to state: recording a version after a *failed* refetch
  // would leave the device holding old data labelled current, and it would never ask again.
  it('does not record the version when the fetch failed', async () => {
    const { store } = storeWith({ [STORAGE_KEY]: storedPayload() })
    store.hydrate()
    store.version = 'v1'

    getMock = async () => {
      throw new Error('offline')
    }
    expect(await store.refreshIfVersionDiffers('v2')).toBe(false)
    expect(store.version).toBe('v1')
  })

  it('stops asking once the caller is known to have no feed', async () => {
    let calls = 0
    getMock = async () => {
      calls++
      throw new HttpError(403, 'forbidden')
    }
    const { store } = storeWith()

    await store.refreshIfVersionDiffers('v1')
    await store.refreshIfVersionDiffers('v2')
    expect(calls).toBe(1)
  })
})

describe('glimtMediaUrl', () => {
  // Addressed by ordinal, never by content hash: a hash in a cached payload would be a forwardable
  // capability that outlives every visibility check (task 302).
  it('builds an ordinal-addressed path', () => {
    expect(glimtMediaUrl('g-1', 0, 'full')).toBe('/api/glimt/items/g-1/media/0')
    expect(glimtMediaUrl('g-1', 2, 'thumb')).toBe('/api/glimt/items/g-1/media/2?variant=thumb')
  })

  it('encodes the id', () => {
    expect(glimtMediaUrl('a/b', 0, 'full')).toBe('/api/glimt/items/a%2Fb/media/0')
  })
})

describe('getters', () => {
  it('sorts newest first defensively', async () => {
    getMock = async () => ({
      glimt: [
        apiGlimt({ id: 'older', created_at: '2026-09-17T20:00:00Z' }),
        apiGlimt({ id: 'newer', created_at: '2026-09-17T22:00:00Z' }),
      ],
    })
    const { store } = storeWith()
    await store.fetch()
    expect(store.newestFirst.map((g) => g.id)).toEqual(['newer', 'older'])
  })

  it('lists the caller’s own glimt', async () => {
    getMock = async () => ({
      glimt: [apiGlimt({ id: 'mine', own: true }), apiGlimt({ id: 'theirs', own: false })],
    })
    const { store } = storeWith()
    await store.fetch()
    expect(store.own.map((g) => g.id)).toEqual(['mine'])
  })
})
