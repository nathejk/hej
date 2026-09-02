import { describe, expect, it } from 'vitest'

import {
  OFFLINE_DATASETS,
  OFFLINE_ORIGIN_BUDGET_BYTES,
  TILE_EVICTION_ZOOM_ORDER,
  evictionOrder,
  offlineDataset,
  totalPlannedBytes,
} from '@/config/offline'

// These tests guard a *decision*, not an implementation. PRD 009 cut the dataset registry, so
// nothing at runtime enforces the priority order — it is a declaration that four independent
// caches are expected to observe. That makes this file the only thing standing between a
// careless future edit and a participant losing their recorded route, which is why the
// invariants below are asserted rather than assumed.

describe('the priority order', () => {
  // The invariant the order exists for. Stated as "no recoverable dataset ranks above an
  // unrecoverable one" rather than "the track is first", so it keeps holding when a second
  // unrecoverable dataset appears.
  it('never places a recoverable dataset above an unrecoverable one', () => {
    const lastUnrecoverable = OFFLINE_DATASETS.reduce(
      (last, d, i) => (d.unrecoverable ? i : last),
      -1,
    )

    for (let i = 0; i < lastUnrecoverable; i++) {
      expect(
        OFFLINE_DATASETS[i].unrecoverable,
        `${OFFLINE_DATASETS[i].id} is recoverable but ranks above an unrecoverable dataset`,
      ).toBe(true)
    }
  })

  it('never offers an unrecoverable dataset for eviction', () => {
    for (const dataset of evictionOrder()) {
      expect(dataset.unrecoverable, dataset.id).toBe(false)
    }
  })

  // Tiles are ~99% of the bytes and everything above them is under 15 MB, so this is the trade
  // the whole order encodes: the map is what gets sacrificed.
  it('evicts map tiles before anything else', () => {
    expect(evictionOrder()[0].id).toBe('tiles')
  })

  it('has no duplicate ids', () => {
    const ids = OFFLINE_DATASETS.map((d) => d.id)
    expect(new Set(ids).size).toBe(ids.length)
  })
})

describe('what the user is told', () => {
  // Two pages describe this data — the profile's readiness section and the privacy page's "what is on
  // your phone" — and they must not describe it differently. Both read these strings, so a dataset
  // added without them would show up on one surface as a blank line and on the other as nothing at all.
  it('gives every dataset a Danish name and a purpose', () => {
    for (const dataset of OFFLINE_DATASETS) {
      expect(dataset.label.trim(), `${dataset.id} label`).not.toBe('')
      expect(dataset.purpose.trim(), `${dataset.id} purpose`).not.toBe('')
      // A whole sentence, not a noun. The privacy page has to answer "what for", and "Kort" does not.
      expect(dataset.purpose.length, `${dataset.id} purpose is too short to explain anything`)
        .toBeGreaterThan(25)
    }
  })

  // The privacy page's own rule, stricter than the rest of the app: readable by a 12-year-old and their
  // parent. Words like "cache" and "tiles" fail that even though they are what the code calls things.
  it('uses no jargon a participant would not recognise', () => {
    const jargon = ['cache', 'tile', 'indexeddb', 'localstorage', 'quota', 'dataset', 'sync']
    for (const dataset of OFFLINE_DATASETS) {
      const text = `${dataset.label} ${dataset.purpose}`.toLowerCase()
      for (const word of jargon) {
        expect(text, `${dataset.id} says "${word}"`).not.toContain(word)
      }
    }
  })
})

describe('the budget', () => {
  it('fits inside the origin ceiling', () => {
    expect(totalPlannedBytes()).toBeLessThanOrEqual(OFFLINE_ORIGIN_BUDGET_BYTES)
  })

  // Not a tautology: it is the check that catches someone raising a single dataset's budget
  // without redoing the sum, which is how an origin quietly grows past the iOS 16 ceiling.
  it('leaves headroom rather than filling the ceiling exactly', () => {
    expect(totalPlannedBytes()).toBeLessThan(OFFLINE_ORIGIN_BUDGET_BYTES * 0.75)
  })

  it('gives every dataset a positive budget', () => {
    for (const dataset of OFFLINE_DATASETS) {
      expect(dataset.budgetBytes, dataset.id).toBeGreaterThan(0)
    }
  })
})

describe('tile eviction order', () => {
  it('discards the highest zoom first', () => {
    const descending = [...TILE_EVICTION_ZOOM_ORDER].sort((a, b) => b - a)
    expect([...TILE_EVICTION_ZOOM_ORDER]).toEqual(descending)
  })

  // z17 is the same 1:25.000 cartography upsampled past its design scale. If it appears here it
  // means something started caching it, which is bytes for no information.
  it('does not mention z17, which is never cached', () => {
    expect(TILE_EVICTION_ZOOM_ORDER).not.toContain(17)
  })
})

describe('offlineDataset', () => {
  it('returns the declared dataset', () => {
    expect(offlineDataset('tiles').kind).toBe('cache-api')
  })

  // Throwing beats returning undefined: every id is a literal in the same file, so a miss is a
  // typo, and a silent undefined would surface as a missing row in the readiness view instead.
  it('throws on an unknown id', () => {
    // @ts-expect-error — deliberately passing an id outside the union.
    expect(() => offlineDataset('rulebook')).toThrow(/unknown offline dataset/)
  })
})
