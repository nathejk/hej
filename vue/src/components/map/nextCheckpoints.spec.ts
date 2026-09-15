import { describe, expect, it } from 'vitest'

import { MAX_ARROW_GROUPS, nextCheckpointGroups } from '@/components/map/nextCheckpoints'
import type { Checkpoint } from '@/stores/checkpoints.store'

function cp(id: string, checkgroup = 'cg-1', over: Partial<Checkpoint> = {}): Checkpoint {
  return {
    id,
    name: id,
    checkgroup,
    sortOrder: 0,
    lat: 56.1,
    lng: 9.5,
    openFrom: 0,
    openUntil: 0,
    openDurationMinutes: 0,
    ...over,
  }
}

const ids = (groups: Checkpoint[][]) => groups.map((g) => g.map((c) => c.id))

describe('nextCheckpointGroups', () => {
  it('groups the posts by checkgroup, in the order given', () => {
    const got = nextCheckpointGroups(
      [cp('a', 'cg-1'), cp('b', 'cg-1'), cp('c', 'cg-2')],
      [],
    )

    expect(ids(got)).toEqual([['a', 'b'], ['c']])
  })

  // The correction this module exists for (task 273). A checkgroup is one leg and its posts are alternatives,
  // so scanning at Post 4A finishes that leg — an arrow to Post 4B would send the patrol somewhere they have
  // no reason to go, and would spend an arrow slot that should show the leg after.
  it('drops the whole group when any of its posts was visited', () => {
    const got = nextCheckpointGroups(
      [cp('4a', 'cg-4'), cp('4b', 'cg-4'), cp('5a', 'cg-5')],
      ['4a'],
    )

    expect(ids(got)).toEqual([['5a']])
  })

  it('keeps a group whose posts are all unvisited', () => {
    const got = nextCheckpointGroups([cp('4a', 'cg-4'), cp('4b', 'cg-4')], ['other'])

    expect(ids(got)).toEqual([['4a', '4b']])
  })

  // Patrols legitimately do legs out of order — a skitse sends them one way, or they walk it their own way.
  // Continuing to point at a leg they have finished is worse than silence.
  it('skips a finished leg even when earlier legs are unfinished', () => {
    const got = nextCheckpointGroups(
      [cp('a', 'cg-1'), cp('b', 'cg-2'), cp('c', 'cg-3')],
      ['b'],
    )

    expect(ids(got)).toEqual([['a'], ['c']])
  })

  // The cap is in *legs*, which is the point of counting groups: three posts that all belong to one leg would
  // otherwise fill the viewport and hide everything behind it.
  it('caps the number of legs, not the number of posts', () => {
    const many = [
      cp('1a', 'cg-1'),
      cp('1b', 'cg-1'),
      cp('1c', 'cg-1'),
      cp('2a', 'cg-2'),
      cp('3a', 'cg-3'),
      cp('4a', 'cg-4'),
    ]

    const got = nextCheckpointGroups(many, [])

    expect(got).toHaveLength(MAX_ARROW_GROUPS)
    expect(ids(got)).toEqual([['1a', '1b', '1c'], ['2a'], ['3a']])
    expect(MAX_ARROW_GROUPS).toBe(3)
  })

  it('respects an explicit limit', () => {
    const got = nextCheckpointGroups([cp('a', 'cg-1'), cp('b', 'cg-2')], [], 1)

    expect(ids(got)).toEqual([['a']])
  })

  // Near the end of a race every leg is done, and that means no arrows rather than a fallback to the nearest
  // post.
  it('returns nothing when every leg is finished', () => {
    const got = nextCheckpointGroups([cp('a', 'cg-1'), cp('b', 'cg-2')], ['a', 'b'])

    expect(got).toEqual([])
  })

  it('returns nothing for an empty list', () => {
    expect(nextCheckpointGroups([], [])).toEqual([])
  })

  // A post can be revealed before its checkgroup is known: the group arrives on a different event
  // (`checkpoint.created`) from the position and name (`checkpoint.updated`), so a catching-up projection can
  // hold one without the other. Such posts stand alone rather than being merged into one nameless group, which
  // would arrow one of them and hide the rest.
  it('treats a post with no checkgroup as its own leg', () => {
    const got = nextCheckpointGroups([cp('a', ''), cp('b', ''), cp('c', 'cg-1')], [])

    expect(ids(got)).toEqual([['a'], ['b'], ['c']])
  })

  // Visiting a group-less post must not take other group-less posts with it — they are unrelated.
  it('does not treat all group-less posts as one leg when one is visited', () => {
    const got = nextCheckpointGroups([cp('a', ''), cp('b', '')], ['a'])

    expect(ids(got)).toEqual([['b']])
  })

  // Route order comes from the BFF, which holds both halves of it. Preserved rather than re-derived: sorting by
  // a post's own order would reorder legs and point the patrol at the wrong one.
  it('does not re-sort by the post’s own order', () => {
    const inRouteOrder = [
      cp('early', 'cg-1', { sortOrder: 5 }),
      cp('late', 'cg-2', { sortOrder: 0 }),
    ]

    expect(ids(nextCheckpointGroups(inRouteOrder, []))).toEqual([['early'], ['late']])
  })
})
