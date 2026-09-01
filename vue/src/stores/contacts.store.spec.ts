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

import { HttpError, NetworkError } from '@/helpers'
import { useContactsStore, type ContactEntry, type ContactsStorage } from '@/stores/contacts.store'
import { useSessionStore } from '@/stores/session.store'

let getMock: (url: string) => Promise<unknown>

// Storage is keyed per profile (task 180), so a test that asserts on what was persisted has to say
// who is signed in. `hej.contacts.v1.<userId>`.
const USER_ID = 'p-viewer'
const STORAGE_KEY = `hej.contacts.v1.${USER_ID}`

// An in-memory stand-in for localStorage, which the node test environment does not provide —
// see vitest.config.ts on why the environment stays `node` and modules take their browser
// surface as an argument instead.
function fakeStorage(initial: Record<string, string> = {}): ContactsStorage & { data: Record<string, string> } {
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

// Builds a store with a fresh fake storage, returning both so tests can assert on what was
// persisted rather than only on state.
//
// Signs a profile in first: the storage key includes the user id, and a store with nobody signed in
// deliberately reads and writes nothing (task 180).
function storeWith(initial: Record<string, string> = {}) {
  const storage = fakeStorage(initial)
  useSessionStore().user = { userId: USER_ID, role: 'bandit' }
  const store = useContactsStore()
  store.storage = storage
  return { store, storage }
}

function entry(over: Partial<ContactEntry> = {}): ContactEntry {
  return {
    id: 'p-1',
    name: 'Bo Bandit',
    population: 'bandit',
    groups: [{ id: 'klan-1', label: 'Klan Ravn', isOwn: true }],
    phone: '+4530000002',
    stillInRace: true,
    portraitVersion: 'thumb-abc',
    ...over,
  }
}

function manifest(entries: ContactEntry[], version = 'v1') {
  return { version, entries }
}

beforeEach(() => {
  setActivePinia(createPinia())
  getMock = () => Promise.reject(new Error('unexpected request'))
})

describe('hydrate', () => {
  it('reads a stored copy so a cold offline start renders', () => {
    const { store } = storeWith({
      [STORAGE_KEY]: JSON.stringify({
        schema: 2,
        version: 'v1',
        syncedAt: 1000,
        expiresAt: Date.now() + 60_000,
        entries: [entry()],
      }),
    })

    store.hydrate()

    expect(store.entries).toHaveLength(1)
    expect(store.version).toBe('v1')
    expect(store.syncedAt).toBe(1000)
    expect(store.expired).toBe(false)
  })

  // The check that actually enforces retention, because it is the only one that runs on a **dormant
  // device**: a phone that has not opened the app since the event, where no purge job, service worker
  // or push will ever run again. A server-issued deadline is the only lever we hold there.
  it('throws away a copy that is past its server-issued deadline', () => {
    const { store, storage } = storeWith({
      [STORAGE_KEY]: JSON.stringify({
        schema: 2,
        version: 'v1',
        syncedAt: 1000,
        expiresAt: Date.now() - 1,
        entries: [entry({ phone: '+4530000002' })],
      }),
    })

    store.hydrate()

    expect(store.entries).toEqual([])
    // Gone from storage too, not merely unread — otherwise the numbers are still on the device for
    // anyone who looks, which is the whole point of the deadline.
    expect(storage.data[STORAGE_KEY]).toBeUndefined()
    // Flagged, so the portrait bytes go with it: half a purge reads as done and is not.
    expect(store.expired).toBe(true)
  })

  it('keeps a copy with no deadline, which is what a disabled TTL produces', () => {
    const { store } = storeWith({
      [STORAGE_KEY]: JSON.stringify({
        schema: 2,
        version: 'v1',
        syncedAt: 1000,
        expiresAt: 0,
        entries: [entry()],
      }),
    })

    store.hydrate()

    expect(store.entries).toHaveLength(1)
  })

  // Schema 1 payloads predate the deadline. Discarded rather than migrated: the data is one request
  // away, and a migration would have to invent an expiry for a payload stored without one — which is
  // exactly the client-computed deadline the server-issued one exists to avoid.
  it('discards a payload from before deadlines existed', () => {
    const { store } = storeWith({
      [STORAGE_KEY]: JSON.stringify({ schema: 1, version: 'v1', syncedAt: 1000, entries: [entry()] }),
    })

    store.hydrate()

    expect(store.entries).toEqual([])
  })

  it('starts empty when nothing is stored', () => {
    const { store } = storeWith()
    store.hydrate()
    expect(store.entries).toEqual([])
    expect(store.hydrated).toBe(true)
  })

  // What comes out of storage is input, not state: a half-written value from a killed tab
  // must not reach the render path.
  it.each([
    ['not json', 'definitely not json'],
    ['wrong schema', JSON.stringify({ schema: 99, version: 'v', syncedAt: 1, entries: [] })],
    ['entries not an array', JSON.stringify({ schema: 1, version: 'v', syncedAt: 1, entries: {} })],
    ['entry missing fields', JSON.stringify({ schema: 1, version: 'v', syncedAt: 1, entries: [{ id: 'x' }] })],
    ['null', 'null'],
  ])('discards malformed storage: %s', (_label, raw) => {
    const { store } = storeWith({ [STORAGE_KEY]: raw })

    expect(() => store.hydrate()).not.toThrow()
    expect(store.entries).toEqual([])
  })

  it('survives storage throwing', () => {
    const { store } = storeWith()
    store.storage = {
      getItem: () => {
        throw new Error('SecurityError: the operation is insecure')
      },
      setItem: () => {},
      removeItem: () => {},
    }

    expect(() => store.hydrate()).not.toThrow()
    expect(store.entries).toEqual([])
  })

  // A platform with no storage at all: the pane works for the session and cannot survive a
  // reload, which is a degradation rather than a failure.
  it('works with no storage available', async () => {
    const { store } = storeWith()
    store.storage = null
    getMock = () => Promise.resolve(manifest([entry()]))

    await store.fetch()
    expect(store.entries).toHaveLength(1)
  })
})

describe('fetch', () => {
  it('stores the payload and records when it synced', async () => {
    getMock = () => Promise.resolve(manifest([entry()]))

    const { store, storage } = storeWith()
    await store.fetch()

    expect(store.entries).toHaveLength(1)
    expect(store.version).toBe('v1')
    expect(store.syncedAt).not.toBeNull()
    expect(storage.data[STORAGE_KEY]).toContain('Bo Bandit')
  })

  it('treats a null entries list as empty rather than crashing', async () => {
    getMock = () => Promise.resolve({ version: 'v1', entries: null })

    const { store, storage } = storeWith()
    await store.fetch()

    expect(store.entries).toEqual([])
  })

  // The property task 160 depends on: a refetch replaces, so a purged phone number really
  // disappears from the device. A merge would keep it forever and make the purge decorative.
  it('replaces rather than merges, so a purged phone number disappears', async () => {
    getMock = () => Promise.resolve(manifest([entry({ phone: '+4530000002' })]))
    const { store, storage } = storeWith()
    await store.fetch()
    expect(store.entries[0].phone).toBe('+4530000002')

    // The member withdraws: same person, no number, marked as out.
    getMock = () =>
      Promise.resolve(manifest([entry({ phone: undefined, stillInRace: false })], 'v2'))
    await store.fetch()

    expect(store.entries).toHaveLength(1)
    expect(store.entries[0].phone).toBeUndefined()
    expect(store.entries[0].stillInRace).toBe(false)
    // And the stored copy must not keep it either.
    expect(storage.data[STORAGE_KEY]).not.toContain('+4530000002')
  })

  // The general form of the test above, added in task 191.
  //
  // That one covers the case we know about — a purged phone number. This covers the *rule*: **no
  // optional field survives a payload that omits it.** Worth stating generally because the next
  // sensitive field will not be `phone`. `crewFunction` reveals where someone is posted and
  // `portraitVersion` is a handle on their photograph; if either could linger after the server
  // stopped sending it, the purge would be decorative for those fields while looking like it worked
  // for the one field that has a test.
  it('lets no omitted field survive a refetch', async () => {
    getMock = () =>
      Promise.resolve(
        manifest([
          entry({ phone: '+4530000002', crewFunction: 'Samarit', portraitVersion: 'abc123' }),
        ]),
      )
    const { store, storage } = storeWith()
    await store.fetch()

    getMock = () =>
      Promise.resolve(
        manifest(
          [entry({ phone: undefined, crewFunction: undefined, portraitVersion: undefined })],
          'v2',
        ),
      )
    await store.fetch()

    const held = store.entries[0]
    expect(held.phone).toBeUndefined()
    expect(held.crewFunction).toBeUndefined()
    expect(held.portraitVersion).toBeUndefined()

    // And nothing lingers in storage either — the copy that survives a reload is the one that
    // matters, and the one an inspection of the device would find.
    const stored = storage.data[STORAGE_KEY] ?? ''
    for (const gone of ['+4530000002', 'Samarit', 'abc123']) {
      expect(stored, `"${gone}" survived in storage`).not.toContain(gone)
    }
  })

  it('drops people who leave the permitted set', async () => {
    getMock = () => Promise.resolve(manifest([entry({ id: 'a' }), entry({ id: 'b' })]))
    const { store, storage } = storeWith()
    await store.fetch()
    expect(store.entries).toHaveLength(2)

    getMock = () => Promise.resolve(manifest([entry({ id: 'a' })], 'v2'))
    await store.fetch()

    expect(store.entries.map((e) => e.id)).toEqual(['a'])
  })

  // The pane's whole premise is working with no signal, so a failed refresh must never
  // discard what we have or throw into the caller.
  it('keeps the previous copy when the network fails', async () => {
    getMock = () => Promise.resolve(manifest([entry()]))
    const { store, storage } = storeWith()
    await store.fetch()

    getMock = () => Promise.reject(new NetworkError('/api/contacts/manifest'))
    await expect(store.fetch()).resolves.toBeUndefined()

    expect(store.entries).toHaveLength(1)
    expect(store.error).not.toBe('')
    expect(store.loading).toBe(false)
  })

  it('clears the error on a later success', async () => {
    getMock = () => Promise.reject(new NetworkError('/api/contacts/manifest'))
    const { store, storage } = storeWith()
    await store.fetch()
    expect(store.error).not.toBe('')

    getMock = () => Promise.resolve(manifest([entry()]))
    await store.fetch()
    expect(store.error).toBe('')
  })

  // A spejder has no contacts pane. That is not a failure to retry, and it must not leave a
  // cached copy behind — a role can change mid-event.
  it('handles 403 as "not for you", clearing any stored copy', async () => {
    getMock = () => Promise.resolve(manifest([entry()]))
    const { store, storage } = storeWith()
    await store.fetch()
    expect(storage.data[STORAGE_KEY]).toBeDefined()

    getMock = () => Promise.reject(new HttpError(403, 'not for you'))
    await store.fetch()

    expect(store.forbidden).toBe(true)
    expect(store.entries).toEqual([])
    expect(store.error).toBe('')
    expect(storage.data[STORAGE_KEY]).toBeUndefined()
  })
})

describe('refreshIfStale', () => {
  it('fetches the payload when nothing is held', async () => {
    const seen: string[] = []
    getMock = (url) => {
      seen.push(url)
      return Promise.resolve(manifest([entry()]))
    }

    const { store, storage } = storeWith()
    expect(await store.refreshIfStale()).toBe(true)
    // Straight to the manifest: no point asking "is my nothing current?".
    expect(seen).toEqual(['/api/contacts/manifest'])
  })

  it('does not refetch when the version is unchanged', async () => {
    getMock = () => Promise.resolve(manifest([entry()], 'v1'))
    const { store, storage } = storeWith()
    await store.fetch()
    const syncedAt = store.syncedAt

    const seen: string[] = []
    getMock = (url) => {
      seen.push(url)
      return Promise.resolve({ version: 'v1' })
    }

    expect(await store.refreshIfStale()).toBe(false)
    expect(seen).toEqual(['/api/contacts/version'])
    // "We checked" is not "we synced": the UI reports the second, so this must not move.
    expect(store.syncedAt).toBe(syncedAt)
  })

  it('refetches when the version differs', async () => {
    getMock = () => Promise.resolve(manifest([entry({ name: 'Old Name' })], 'v1'))
    const { store, storage } = storeWith()
    await store.fetch()

    const seen: string[] = []
    getMock = (url) => {
      seen.push(url)
      if (url === '/api/contacts/version') return Promise.resolve({ version: 'v2' })
      return Promise.resolve(manifest([entry({ name: 'New Name' })], 'v2'))
    }

    expect(await store.refreshIfStale()).toBe(true)
    expect(seen).toEqual(['/api/contacts/version', '/api/contacts/manifest'])
    expect(store.entries[0].name).toBe('New Name')
  })

  it('keeps the copy when the version check fails offline', async () => {
    getMock = () => Promise.resolve(manifest([entry()], 'v1'))
    const { store, storage } = storeWith()
    await store.fetch()

    getMock = () => Promise.reject(new NetworkError('/api/contacts/version'))
    expect(await store.refreshIfStale()).toBe(false)

    expect(store.entries).toHaveLength(1)
    expect(store.version).toBe('v1')
  })
})

describe('groupViews', () => {
  it('groups by innermost group and puts the own group first', async () => {
    getMock = () =>
      Promise.resolve(
        manifest([
          entry({ id: 'a', name: 'Anders', groups: [{ id: 'klan-2', label: 'Klan Ulv', isOwn: false }] }),
          entry({ id: 'b', name: 'Bo', groups: [{ id: 'klan-1', label: 'Klan Ravn', isOwn: true }] }),
          entry({ id: 'c', name: 'Cecilie', groups: [{ id: 'klan-1', label: 'Klan Ravn', isOwn: true }] }),
        ]),
      )

    const { store, storage } = storeWith()
    await store.fetch()

    const views = store.groupViews
    expect(views).toHaveLength(2)
    expect(views[0].group.label).toBe('Klan Ravn')
    expect(views[0].group.isOwn).toBe(true)
    expect(views[0].entries.map((e) => e.name)).toEqual(['Bo', 'Cecilie'])
    expect(views[1].group.label).toBe('Klan Ulv')
  })

  it('keeps the same person in two populations as two rows', async () => {
    // A crew bandit: listed among banditter and among crew. Both lists answer "who is out as
    // what", so this is intended rather than a duplicate to collapse.
    getMock = () =>
      Promise.resolve(
        manifest([
          entry({ id: 'p-crew-bandit', population: 'bandit', groups: [{ id: 'klan-1', label: 'Klan Ravn', isOwn: false }] }),
          entry({ id: 'p-crew-bandit', population: 'crew', groups: [{ id: 'crew', label: 'Crew', isOwn: true }] }),
        ]),
      )

    const { store, storage } = storeWith()
    await store.fetch()

    expect(store.groupViews).toHaveLength(2)
    // But a lookup by id sees one person, so a favourite cannot be duplicated.
    expect(store.byId.size).toBe(1)
  })

  it('ignores an entry with no group rather than inventing one', async () => {
    getMock = () => Promise.resolve(manifest([entry({ groups: [] })]))
    const { store, storage } = storeWith()
    await store.fetch()
    expect(store.groupViews).toEqual([])
  })

  it('sorts Danish labels correctly', async () => {
    getMock = () =>
      Promise.resolve(
        manifest([
          entry({ id: 'a', groups: [{ id: '1', label: 'Ærlige Ulve', isOwn: false }] }),
          entry({ id: 'b', groups: [{ id: '2', label: 'Bjørne', isOwn: false }] }),
        ]),
      )

    const { store, storage } = storeWith()
    await store.fetch()
    // Æ sorts after B in Danish, not before as a naive codepoint sort would have it.
    expect(store.groupViews.map((v) => v.group.label)).toEqual(['Bjørne', 'Ærlige Ulve'])
  })
})
