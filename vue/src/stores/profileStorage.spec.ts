import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

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

import { dropLegacyKey, profileKey } from '@/helpers/profileStorage'
import { useContactsStore, type ContactEntry, type ContactsStorage } from '@/stores/contacts.store'
import { useFavouritesStore } from '@/stores/favourites.store'
import { useSessionStore } from '@/stores/session.store'

let getMock: (url: string) => Promise<unknown>

// Per-profile storage (PRD 012 §8, task 180).
//
// Several profiles share one phone number and therefore one device. Everything cached about "who I
// can see" is per profile — this file is the assertion that they cannot read each other's, which was
// not true before the profile switcher was designed.

function fakeStorage(initial: Record<string, string> = {}) {
  const data = { ...initial }
  const storage: ContactsStorage & { data: Record<string, string> } = {
    data,
    getItem: (k) => (k in data ? data[k] : null),
    setItem: (k, v) => {
      data[k] = v
    },
    removeItem: (k) => {
      delete data[k]
    },
  }
  return storage
}

function entry(over: Partial<ContactEntry> = {}): ContactEntry {
  return {
    id: 'p-1',
    name: 'Bo Bandit',
    population: 'bandit',
    groups: [{ id: 'klan-1', label: 'Klan Ravn', isOwn: true }],
    phone: '+4530000002',
    stillInRace: true,
    ...over,
  }
}

function signIn(userId: string) {
  useSessionStore().user = { userId, role: 'bandit' }
}

beforeEach(() => {
  setActivePinia(createPinia())
  getMock = () => Promise.resolve({ version: 'v1', entries: [entry()] })
})

describe('profileKey', () => {
  it('scopes a base key to a user', () => {
    expect(profileKey('hej.contacts.v1', 'abc')).toBe('hej.contacts.v1.abc')
  })

  // There is no device-wide fallback on purpose: writing one would put a profile's data back under a
  // key the next profile reads, which is the bug this exists to prevent.
  it('returns null without a user', () => {
    expect(profileKey('hej.contacts.v1', null)).toBeNull()
    expect(profileKey('hej.contacts.v1', undefined)).toBeNull()
    expect(profileKey('hej.contacts.v1', '')).toBeNull()
  })
})

describe('dropLegacyKey', () => {
  it('removes the pre-scoping key', () => {
    const storage = fakeStorage({ 'hej.contacts.v1': '{"schema":1}' })
    dropLegacyKey(storage, 'hej.contacts.v1')
    expect(storage.data['hej.contacts.v1']).toBeUndefined()
  })

  it('tolerates missing or throwing storage', () => {
    expect(() => dropLegacyKey(null, 'x')).not.toThrow()
    expect(() =>
      dropLegacyKey(
        {
          getItem: () => null,
          setItem: () => {},
          removeItem: () => {
            throw new Error('SecurityError')
          },
        },
        'x',
      ),
    ).not.toThrow()
  })
})

describe('contacts directory per profile', () => {
  it('writes under a key that includes the profile', async () => {
    signIn('user-a')
    const storage = fakeStorage()
    const store = useContactsStore()
    store.storage = storage

    await store.fetch()

    expect(Object.keys(storage.data)).toEqual(['hej.contacts.v1.user-a'])
  })

  // The bug this task fixes: sign out, sign in as a sibling on the same handset, and the previous
  // profile's cached directory was what you saw.
  it('does not read another profile\'s cache', async () => {
    const storage = fakeStorage()

    signIn('user-a')
    const a = useContactsStore()
    a.storage = storage
    getMock = () => Promise.resolve({ version: 'va', entries: [entry({ name: 'A Colleague' })] })
    await a.fetch()

    // A different profile on the same device, with a fresh store instance.
    setActivePinia(createPinia())
    signIn('user-b')
    const b = useContactsStore()
    b.storage = storage
    b.hydrate()

    expect(b.entries).toEqual([])
    expect(b.version).toBe('')
    expect(b.syncedAt).toBeNull()
  })

  // Keying rather than clearing means switching back finds your own cache intact, which matters for
  // the parent-with-two-children case where switching is frequent.
  it('finds the earlier cache again on switching back', async () => {
    const storage = fakeStorage()

    signIn('user-a')
    const a = useContactsStore()
    a.storage = storage
    getMock = () => Promise.resolve({ version: 'va', entries: [entry({ name: 'A Colleague' })] })
    await a.fetch()

    setActivePinia(createPinia())
    signIn('user-b')
    const b = useContactsStore()
    b.storage = storage
    getMock = () => Promise.resolve({ version: 'vb', entries: [entry({ name: 'B Colleague' })] })
    await b.fetch()

    setActivePinia(createPinia())
    signIn('user-a')
    const backToA = useContactsStore()
    backToA.storage = storage
    backToA.hydrate()

    expect(backToA.entries.map((e) => e.name)).toEqual(['A Colleague'])
    expect(backToA.version).toBe('va')
    // Both caches coexist; neither overwrote the other.
    expect(Object.keys(storage.data).sort()).toEqual([
      'hej.contacts.v1.user-a',
      'hej.contacts.v1.user-b',
    ])
  })

  it('reads and writes nothing with nobody signed in', async () => {
    const storage = fakeStorage()
    const store = useContactsStore()
    store.storage = storage

    store.hydrate()
    await store.fetch()

    // The fetch still populates memory for the session; it simply has nowhere legitimate to
    // persist it.
    expect(Object.keys(storage.data)).toEqual([])
  })

  it('removes the pre-scoping device-wide cache on first hydrate', () => {
    signIn('user-a')
    const storage = fakeStorage({
      'hej.contacts.v1': JSON.stringify({ schema: 1, version: 'old', syncedAt: 1, entries: [entry()] }),
    })
    const store = useContactsStore()
    store.storage = storage

    store.hydrate()

    // Deleted, not merely ignored: it is a readable cache of other members' names and numbers.
    expect(storage.data['hej.contacts.v1']).toBeUndefined()
    expect(store.entries).toEqual([])
  })
})

describe('favourites per profile', () => {
  it('does not inherit another profile\'s favourites', () => {
    const storage = fakeStorage()

    signIn('user-a')
    const a = useFavouritesStore()
    a.hydrate(storage)
    a.toggle('colleague-1')
    expect(a.ids).toEqual(['colleague-1'])

    setActivePinia(createPinia())
    signIn('user-b')
    const b = useFavouritesStore()
    b.hydrate(storage)

    expect(b.ids).toEqual([])
    expect(b.has('colleague-1')).toBe(false)
  })

  it('keeps each profile\'s favourites under its own key', () => {
    const storage = fakeStorage()

    signIn('user-a')
    useFavouritesStore().hydrate(storage)
    useFavouritesStore().toggle('a-fav')

    setActivePinia(createPinia())
    signIn('user-b')
    useFavouritesStore().hydrate(storage)
    useFavouritesStore().toggle('b-fav')

    expect(storage.data['hej.contacts.favourites.v1.user-a']).toBe(JSON.stringify(['a-fav']))
    expect(storage.data['hej.contacts.favourites.v1.user-b']).toBe(JSON.stringify(['b-fav']))
  })

  it('removes the pre-scoping device-wide favourites on first hydrate', () => {
    signIn('user-a')
    const storage = fakeStorage({
      'hej.contacts.favourites.v1': JSON.stringify(['someone-elses-favourite']),
    })

    const store = useFavouritesStore()
    store.hydrate(storage)

    expect(storage.data['hej.contacts.favourites.v1']).toBeUndefined()
    expect(store.ids).toEqual([])
  })

  it('persists nothing with nobody signed in', () => {
    const storage = fakeStorage()
    const store = useFavouritesStore()
    store.hydrate(storage)

    store.toggle('colleague-1')

    // In memory for the session, but not written anywhere another profile could read.
    expect(store.has('colleague-1')).toBe(true)
    expect(Object.keys(storage.data)).toEqual([])
  })
})
