import { describe, expect, it } from 'vitest'

import { foldForSearch, searchContacts } from '@/helpers/contactSearch'
import type { ContactEntry } from '@/stores/contacts.store'

function entry(over: Partial<ContactEntry> = {}): ContactEntry {
  return {
    id: 'p-1',
    name: 'Bo Bandit',
    population: 'bandit',
    groups: [{ id: 'klan-1', label: 'Klan Ravn', isOwn: false }],
    phone: '+4530000002',
    stillInRace: true,
    ...over,
  }
}

describe('foldForSearch', () => {
  it('lower-cases', () => {
    expect(foldForSearch('Bo BANDIT')).toBe('bo bandit')
  })

  // Danish names are the norm in this data, so accent-sensitive matching would fail on the
  // common case rather than an edge one.
  it.each([
    ['Søren', 'soren'],
    ['Ærlige', 'aerlige'],
    ['Åge', 'age'],
    ['Bjørn', 'bjorn'],
    ['Malmö', 'malmo'],
    ['José', 'jose'],
    ['Müller', 'muller'],
  ])('folds %s to %s', (input, want) => {
    expect(foldForSearch(input)).toBe(want)
  })

  it('leaves ascii alone', () => {
    expect(foldForSearch('patrulje 138')).toBe('patrulje 138')
  })
})

describe('searchContacts', () => {
  const people = [
    entry({ id: 'a', name: 'Søren Sørensen', groups: [{ id: 'k1', label: 'Klan Ravn', isOwn: true }] }),
    entry({ id: 'b', name: 'Bo Bandit', groups: [{ id: 'k2', label: 'Klan Ulv', isOwn: false }] }),
    entry({
      id: 'c',
      name: 'Sara Samarit',
      population: 'crew',
      crewFunction: 'Samaritter',
      groups: [{ id: 'crew', label: 'Crew', isOwn: false }],
      phone: '+4530000005',
    }),
  ]

  it('returns nothing for an empty or whitespace query', () => {
    // Not "everything": the view shows its grouped sections in that case, and returning the
    // whole directory would make the two states indistinguishable to it.
    expect(searchContacts(people, '')).toEqual([])
    expect(searchContacts(people, '   ')).toEqual([])
  })

  it('matches on name', () => {
    expect(searchContacts(people, 'bo').map((e) => e.id)).toEqual(['b'])
  })

  it('matches a name typed without Danish letters', () => {
    expect(searchContacts(people, 'soren').map((e) => e.id)).toEqual(['a'])
    expect(searchContacts(people, 'Søren').map((e) => e.id)).toEqual(['a'])
  })

  it('matches on group label', () => {
    expect(searchContacts(people, 'ulv').map((e) => e.id)).toEqual(['b'])
  })

  it('matches on crew function', () => {
    expect(searchContacts(people, 'samaritter').map((e) => e.id)).toEqual(['c'])
  })

  it('matches a partial name mid-string', () => {
    expect(searchContacts(people, 'sam').map((e) => e.id)).toEqual(['c'])
  })

  // "Who was that missed call from?" is a real question during an event, and the alternative is
  // scrolling.
  it('matches on phone number regardless of spacing', () => {
    expect(searchContacts(people, '30000005').map((e) => e.id)).toEqual(['c'])
    expect(searchContacts(people, '30 00 00 05').map((e) => e.id)).toEqual(['c'])
    expect(searchContacts(people, '+45 30 00 00 05').map((e) => e.id)).toEqual(['c'])
  })

  it('does not treat a two-digit query as a number search', () => {
    // Too short to be a useful number match, and it would otherwise match nearly everyone.
    expect(searchContacts(people, '30').map((e) => e.id)).toEqual([])
  })

  it('sorts favourites first, then by name', () => {
    const isFav = (id: string) => id === 'c'
    const got = searchContacts(people, 's', isFav)
    expect(got.map((e) => e.id)).toEqual(['c', 'a'])
  })

  it('sorts Danish names correctly', () => {
    const danish = [
      entry({ id: 'ae', name: 'Æbbe Nielsen' }),
      entry({ id: 'b', name: 'Bent Nielsen' }),
      entry({ id: 'o', name: 'Øjvind Nielsen' }),
    ]
    // Æ and Ø sort after Z in Danish, not near A and O as a codepoint sort would have it.
    expect(searchContacts(danish, 'nielsen').map((e) => e.id)).toEqual(['b', 'ae', 'o'])
  })

  it('returns an empty list when nothing matches', () => {
    expect(searchContacts(people, 'zzzz')).toEqual([])
  })

  it('handles an entry with no phone, group or crew function', () => {
    const sparse = [entry({ id: 'sparse', name: 'Ukendt', phone: undefined, groups: [] })]
    expect(() => searchContacts(sparse, 'ukendt')).not.toThrow()
    expect(searchContacts(sparse, 'ukendt').map((e) => e.id)).toEqual(['sparse'])
  })

  it('does not mutate the input array', () => {
    const input = [...people]
    searchContacts(input, 's')
    expect(input.map((e) => e.id)).toEqual(people.map((e) => e.id))
  })
})
