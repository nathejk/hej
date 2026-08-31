import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useContactsStore, type ContactEntry, type ContactsStorage } from '@/stores/contacts.store'
import { useFavouritesStore } from '@/stores/favourites.store'

const STORAGE_KEY = 'hej.contacts.favourites.v1'

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

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('favourites', () => {
  it('starts empty and toggles on', () => {
    const store = useFavouritesStore()
    store.hydrate(fakeStorage())

    expect(store.count).toBe(0)
    store.toggle('p-1')
    expect(store.has('p-1')).toBe(true)
    expect(store.count).toBe(1)
  })

  it('toggles off again', () => {
    const store = useFavouritesStore()
    store.hydrate(fakeStorage())

    store.toggle('p-1')
    store.toggle('p-1')
    expect(store.has('p-1')).toBe(false)
  })

  it('persists ids only, never names or numbers', () => {
    const storage = fakeStorage()
    const store = useFavouritesStore()
    store.hydrate(storage)

    store.toggle('p-1')

    const raw = storage.data[STORAGE_KEY]
    expect(raw).toBe(JSON.stringify(['p-1']))
    // A stale copy of a name or number could outlive the user's access to that person, so
    // nothing but the id is written.
    expect(raw).not.toContain('Bo Bandit')
    expect(raw).not.toContain('+45')
  })

  it('hydrates from storage', () => {
    const store = useFavouritesStore()
    store.hydrate(fakeStorage({ [STORAGE_KEY]: JSON.stringify(['a', 'b']) }))

    expect(store.ids).toEqual(['a', 'b'])
  })

  it('keeps the order favourites were added in', () => {
    const store = useFavouritesStore()
    store.hydrate(fakeStorage())

    store.toggle('b')
    store.toggle('a')
    store.toggle('c')
    expect(store.ids).toEqual(['b', 'a', 'c'])
  })

  // What comes out of storage is input, not state.
  it.each([
    ['not json', 'nonsense'],
    ['not an array', JSON.stringify({ a: 1 })],
    ['null', 'null'],
  ])('discards malformed storage: %s', (_label, raw) => {
    const store = useFavouritesStore()
    expect(() => store.hydrate(fakeStorage({ [STORAGE_KEY]: raw }))).not.toThrow()
    expect(store.ids).toEqual([])
  })

  // One bad element should cost that element, not the whole list.
  it('filters non-string entries rather than dropping everything', () => {
    const store = useFavouritesStore()
    store.hydrate(fakeStorage({ [STORAGE_KEY]: JSON.stringify(['a', 42, null, '', 'b']) }))
    expect(store.ids).toEqual(['a', 'b'])
  })

  it('works with no storage at all', () => {
    const store = useFavouritesStore()
    store.hydrate(null)

    expect(() => store.toggle('p-1')).not.toThrow()
    expect(store.has('p-1')).toBe(true)
  })

  it('survives storage throwing on write', () => {
    const store = useFavouritesStore()
    store.hydrate({
      getItem: () => null,
      setItem: () => {
        throw new Error('QuotaExceededError')
      },
      removeItem: () => {},
    })

    expect(() => store.toggle('p-1')).not.toThrow()
    expect(store.has('p-1')).toBe(true)
  })
})

describe('pruneAgainstDirectory', () => {
  it('drops a favourite the user may no longer see', () => {
    const contacts = useContactsStore()
    contacts.entries = [entry({ id: 'still-visible' })]

    const storage = fakeStorage()
    const store = useFavouritesStore()
    store.hydrate(storage)
    store.toggle('still-visible')
    store.toggle('gone-from-scope')

    store.pruneAgainstDirectory()

    expect(store.ids).toEqual(['still-visible'])
    // And the pruning must reach storage, or it comes back on the next launch.
    expect(storage.data[STORAGE_KEY]).toBe(JSON.stringify(['still-visible']))
  })

  // A withdrawn member is still in the manifest, with a marking and no number. Losing them from
  // favourites the moment they go home is exactly when somebody might be looking for them.
  it('keeps a favourite who has left the race', () => {
    const contacts = useContactsStore()
    contacts.entries = [entry({ id: 'withdrawn', stillInRace: false, phone: undefined })]

    const store = useFavouritesStore()
    store.hydrate(fakeStorage())
    store.toggle('withdrawn')

    store.pruneAgainstDirectory()

    expect(store.has('withdrawn')).toBe(true)
  })

  // An offline start that has not synced yet must not be read as "you may see nobody".
  it('does nothing while the directory is empty', () => {
    const contacts = useContactsStore()
    contacts.entries = []

    const store = useFavouritesStore()
    store.hydrate(fakeStorage())
    store.toggle('a')
    store.toggle('b')

    store.pruneAgainstDirectory()

    expect(store.ids).toEqual(['a', 'b'])
  })

  it('does not rewrite storage when nothing changed', () => {
    const contacts = useContactsStore()
    contacts.entries = [entry({ id: 'a' })]

    const storage = fakeStorage()
    const store = useFavouritesStore()
    store.hydrate(storage)
    store.toggle('a')

    const before = storage.data[STORAGE_KEY]
    store.pruneAgainstDirectory()
    expect(storage.data[STORAGE_KEY]).toBe(before)
  })

  it('resolves a person listed in two populations as one favourite', () => {
    const contacts = useContactsStore()
    // A crew bandit: two entries, one person.
    contacts.entries = [
      entry({ id: 'crew-bandit', population: 'bandit' }),
      entry({ id: 'crew-bandit', population: 'crew' }),
    ]

    const store = useFavouritesStore()
    store.hydrate(fakeStorage())
    store.toggle('crew-bandit')

    store.pruneAgainstDirectory()

    expect(store.ids).toEqual(['crew-bandit'])
  })
})
