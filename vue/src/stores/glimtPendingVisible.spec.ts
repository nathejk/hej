import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

// A queued glimt is visible in the feed (PRD 019 §5, task 325).
//
// # The bug this file exists to prevent
//
// Found on an iPhone, 2026-09-17, in airplane mode. Posting a glimt with two photographs produced a
// screen carrying **both** of these at once:
//
//     Et glimt venter på nettet.
//     Ingen glimt endnu
//
// The outbox was working perfectly — the post survived a force-quit and drained on reconnect. What
// was missing is that the feed only ever rendered what it had *fetched*, and reported the queue as a
// bare count above it. So the app contradicted itself on one screen while a member stood in a field
// wondering where their photographs had gone.
//
// PRD 019 §5 is explicit and was not met: *"The glimt appears immediately in their own feed"*, and
// *"the **entry** is visibly venter på nettet, never silently dropped."* A counter is not an entry.
//
// # What is asserted here
//
// The store's `feed` and `isEmpty` getters, because they are where the contradiction lived. Component
// rendering cannot be tested in this suite (node, no DOM, nothing mounted), so the guard is placed on
// the data the view reads — which is also the level the bug was actually at.

vi.mock('@/helpers', async () => {
  const actual = await vi.importActual<typeof import('@/helpers')>('@/helpers')
  return { ...actual, fetchWrapper: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }
})

import { useGlimtStore, type Glimt } from '@/stores/glimt.store'

function fetched(id: string, createdAt: number): Glimt {
  return {
    id,
    hold: { number: '42', name: 'Ørnene', group: 'spejder' },
    own: false,
    audience: 'group',
    caption: '',
    createdAt,
    media: [{ ordinal: 0, kind: 'image', width: 1600, height: 1200, durationMs: 0, hasThumb: true }],
    hidden: false,
  }
}

function queued(id: string, createdAt: number): Glimt {
  return {
    id,
    hold: { number: '', name: '', group: '' },
    own: true,
    audience: 'group',
    caption: 'offline',
    createdAt,
    media: [
      {
        ordinal: 0,
        kind: 'image',
        width: 0,
        height: 0,
        durationMs: 0,
        hasThumb: false,
        localUrl: `blob:${id}-0`,
      },
    ],
    hidden: false,
    pending: true,
    attempts: 0,
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('the feed shows queued glimt', () => {
  // The exact state from the screenshot: nothing fetched, one thing queued.
  it('is not empty when something is queued', () => {
    const store = useGlimtStore()
    store.glimt = []
    store.pendingGlimt = [queued('draft-1', 1000)]

    expect(
      store.isEmpty,
      'the feed reported itself empty while a glimt was waiting to upload — this is the bug that ' +
        'put "Ingen glimt endnu" directly under "Et glimt venter på nettet"',
    ).toBe(false)
    expect(store.feed).toHaveLength(1)
  })

  it('is empty only when nothing is cached and nothing is queued', () => {
    const store = useGlimtStore()
    store.glimt = []
    store.pendingGlimt = []
    expect(store.isEmpty).toBe(true)
  })

  it('includes queued and fetched glimt together', () => {
    const store = useGlimtStore()
    store.glimt = [fetched('g-1', 2000)]
    store.pendingGlimt = [queued('draft-1', 1000)]
    expect(store.feed.map((g) => g.id)).toEqual(['draft-1', 'g-1'])
  })

  // Queued first regardless of timestamp. A member who has just posted is looking for *their*
  // photograph, and it is the one thing on screen that may still need them — a retry, or discarding.
  // Interleaving by createdAt would bury it under a hold's afternoon.
  it('puts queued glimt first even when they are older', () => {
    const store = useGlimtStore()
    store.glimt = [fetched('g-new', 9000)]
    store.pendingGlimt = [queued('draft-old', 1)]
    expect(store.feed[0].id).toBe('draft-old')
  })

  it('keeps the fetched feed newest-first below the queue', () => {
    const store = useGlimtStore()
    store.glimt = [fetched('g-old', 1000), fetched('g-new', 5000)]
    store.pendingGlimt = [queued('draft-1', 3000)]
    expect(store.feed.map((g) => g.id)).toEqual(['draft-1', 'g-new', 'g-old'])
  })
})

describe('a queued glimt carries its bytes locally', () => {
  // It has no server identity, so /api/glimt/items/{draftId}/media/0 would 404. The blob URL is the
  // only way its photograph can be drawn, which is why `GlimtMedia.localUrl` exists at all.
  it('has a local url on every item that has not uploaded', () => {
    const entry = queued('draft-1', 1000)
    for (const item of entry.media) {
      expect(item.localUrl, 'a queued item with no local url cannot be rendered').toBeTruthy()
      expect(item.localUrl).toMatch(/^blob:/)
    }
  })

  it('is marked pending, so the card can say so and withhold server actions', () => {
    expect(queued('draft-1', 1000).pending).toBe(true)
  })

  // No attribution until the server freezes it at creation (PRD 019 §6). The card falls back to
  // "Dit hold" via `own`, rather than this store guessing a patrulje it might publish differently.
  it('carries no hold attribution yet', () => {
    const entry = queued('draft-1', 1000)
    expect(entry.hold.number).toBe('')
    expect(entry.own).toBe(true)
  })
})

describe('releasePendingUrls', () => {
  // Each object URL pins a photograph in memory until revoked, and the queue can hold tens of
  // megabytes. Without this a member who queues several posts in a coverage hole carries all of them
  // for the life of the tab.
  it('revokes every url the projection holds', () => {
    const revoked: string[] = []
    const original = URL.revokeObjectURL
    URL.revokeObjectURL = (url: string) => revoked.push(url) as unknown as void

    try {
      const store = useGlimtStore()
      store.pendingGlimt = [queued('draft-1', 1), queued('draft-2', 2)]
      store.releasePendingUrls()
      expect(revoked).toEqual(['blob:draft-1-0', 'blob:draft-2-0'])
    } finally {
      URL.revokeObjectURL = original
    }
  })

  it('does not throw when there is nothing to release', () => {
    const store = useGlimtStore()
    store.pendingGlimt = []
    expect(() => store.releasePendingUrls()).not.toThrow()
  })
})
