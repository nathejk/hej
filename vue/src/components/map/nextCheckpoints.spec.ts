import { describe, expect, it } from 'vitest'

import { MAX_ARROWS, nextCheckpoints } from '@/components/map/nextCheckpoints'
import type { Checkpoint } from '@/stores/checkpoints.store'

function cp(id: string, over: Partial<Checkpoint> = {}): Checkpoint {
  return {
    id,
    name: id,
    checkgroup: 'cg-1',
    sortOrder: 0,
    lat: 56.1,
    lng: 9.5,
    openFrom: 0,
    openUntil: 0,
    openDurationMinutes: 0,
    ...over,
  }
}

describe('nextCheckpoints', () => {
  it('takes the earliest unvisited posts in the order given', () => {
    const got = nextCheckpoints([cp('a'), cp('b'), cp('c'), cp('d')], [])

    expect(got.map((c) => c.id)).toEqual(['a', 'b', 'c'])
  })

  it('skips posts already visited', () => {
    const got = nextCheckpoints([cp('a'), cp('b'), cp('c'), cp('d')], ['a', 'b'])

    expect(got.map((c) => c.id)).toEqual(['c', 'd'])
  })

  // A patrol may reach posts out of order — a skitse sends them to one post before another, or they simply
  // walk it their own way. Continuing to point at somewhere they have already stood would be worse than
  // silence.
  it('skips a visited post even when earlier ones are unvisited', () => {
    const got = nextCheckpoints([cp('a'), cp('b'), cp('c')], ['b'])

    expect(got.map((c) => c.id)).toEqual(['a', 'c'])
  })

  // The viewport is a phone screen at night. Four arrows is decoration rather than instruction.
  it('caps the count', () => {
    const many = ['a', 'b', 'c', 'd', 'e', 'f'].map((id) => cp(id))

    expect(nextCheckpoints(many, [])).toHaveLength(MAX_ARROWS)
    expect(MAX_ARROWS).toBe(3)
  })

  it('respects an explicit limit', () => {
    const got = nextCheckpoints([cp('a'), cp('b'), cp('c')], [], 1)

    expect(got.map((c) => c.id)).toEqual(['a'])
  })

  // Everything visited is a real state near the end of a race, and it means no arrows rather than a fallback
  // to the nearest post.
  it('returns nothing when every post has been visited', () => {
    expect(nextCheckpoints([cp('a'), cp('b')], ['a', 'b'])).toEqual([])
  })

  it('returns nothing for an empty list', () => {
    expect(nextCheckpoints([], [])).toEqual([])
  })

  // Route order comes from the BFF, which is the only side holding both halves of it. Preserved here rather
  // than re-derived: sorting by the post's own sortOrder alone would reorder posts across checkgroups and
  // point the patrol at the wrong leg.
  it('does not re-sort by the post’s own order', () => {
    // Route order: cg-1 then cg-2. Within cg-2 the post's own order is lower, which a naive sort would
    // promote to first.
    const inRouteOrder = [
      cp('early', { checkgroup: 'cg-1', sortOrder: 5 }),
      cp('late', { checkgroup: 'cg-2', sortOrder: 0 }),
    ]

    const got = nextCheckpoints(inRouteOrder, [])

    expect(got.map((c) => c.id)).toEqual(['early', 'late'])
  })
})
