import { createPinia, setActivePinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { beforeEach, describe, expect, it, vi } from 'vitest'

// The moderation queue is the one payload in this app that names the author of a glimt, and it must
// never reach disk (PRD 019 §0b, §6, tasks 308/309).
//
// # Why this deserves its own file
//
// Everywhere else the guarantee is structural and therefore free: the BFF projects the author out of
// the response, so no client can leak what it never received. The queue is the deliberate exception —
// a report cannot be answered against an anonymous author — which means this is the one place where
// the guarantee depends on the client behaving. `profileNotCached.spec.ts` sets the precedent for
// writing that kind of promise down as a test.
//
// The natural way to break it is a well-meant change: folding the queue into `glimt` to reuse the
// feed's caching, or adding it to the stored payload so a moderator can triage offline. Whoever does
// that lands here, which is the point.

vi.mock('@/helpers', async () => {
  const actual = await vi.importActual<typeof import('@/helpers')>('@/helpers')
  return {
    ...actual,
    fetchWrapper: {
      get: (url: string) => getMock(url),
      post: (url: string) => postMock(url),
      put: vi.fn(),
      delete: vi.fn(),
    },
  }
})

import { HttpError } from '@/helpers'
import { useGlimtStore, type GlimtStorage } from '@/stores/glimt.store'
import { useSessionStore } from '@/stores/session.store'

let getMock: (url: string) => Promise<unknown>
let postMock: (url: string) => Promise<unknown>

const USER_ID = 'p-moderator'
const STORAGE_KEY = `hej.glimt.v1.${USER_ID}`

const AUTHOR_NAME = 'Astrid Mortensen'
const AUTHOR_ID = 'p-author-1'

function fakeStorage(): GlimtStorage & { data: Record<string, string> } {
  const data: Record<string, string> = {}
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

// One queue entry as the BFF sends it: the feed shape plus the four moderation-only fields.
function apiQueueEntry(over: Record<string, unknown> = {}) {
  return {
    id: 'g-reported',
    hold: { number: '42', name: 'Ørnene', group: 'spejder' },
    own: false,
    audience: 'public',
    caption: 'ved posten',
    created_at: '2026-09-17T21:00:00Z',
    media: [{ ordinal: 0, kind: 'image', width: 1600, height: 1200, has_thumb: true }],
    hidden: false,
    author_name: AUTHOR_NAME,
    author_person_id: AUTHOR_ID,
    report_count: 2,
    hidden_by: '',
    ...over,
  }
}

function storeWith() {
  const storage = fakeStorage()
  useSessionStore().user = { userId: USER_ID, role: 'crew' }
  const store = useGlimtStore()
  store.storage = storage
  return { store, storage }
}

beforeEach(() => {
  setActivePinia(createPinia())
  getMock = () => Promise.resolve({ glimt: [apiQueueEntry()] })
  postMock = () => Promise.resolve(undefined)
})

describe('the moderation queue', () => {
  it('maps the author fields the BFF sends only to moderators', async () => {
    const { store } = storeWith()
    expect(await store.fetchModeration()).toBe(true)

    const [entry] = store.moderationQueue
    expect(entry.authorName).toBe(AUTHOR_NAME)
    expect(entry.authorPersonId).toBe(AUTHOR_ID)
    expect(entry.reportCount).toBe(2)
    // And the ordinary glimt shape came through the same mapper, so the queue can reuse GlimtCard.
    expect(entry.hold.name).toBe('Ørnene')
    expect(entry.audience).toBe('public')
  })

  // The load-bearing one.
  it('is not written to device storage, even after a persist', async () => {
    const { store, storage } = storeWith()
    await store.fetchModeration()
    // Force the write path that *does* persist, so this is not passing merely because nothing was
    // saved at all.
    store.glimt = [...store.moderationQueue].map((g) => ({ ...g }))
    await store.persist()

    const written = storage.data[STORAGE_KEY]
    expect(written, 'nothing was persisted, so this test proved nothing').toBeTruthy()
    expect(written).not.toContain(AUTHOR_NAME)
    expect(written).not.toContain(AUTHOR_ID)
    expect(written).not.toContain('authorName')
    expect(written).not.toContain('report_count')
    expect(written).not.toContain('reportCount')
  })

  it('does not survive leaving the screen', async () => {
    const { store } = storeWith()
    await store.fetchModeration()
    expect(store.moderationQueue).toHaveLength(1)

    store.clearModeration()
    expect(store.moderationQueue).toEqual([])
    expect(store.moderationForbidden).toBe(false)
  })

  // A 403 is the correct answer for a caller who should not have been offered the page — an
  // assignment revoked mid-session lands here — so it must not read as a failure with a retry button.
  it('treats a refusal as a state rather than an error', async () => {
    const { store } = storeWith()
    getMock = () => Promise.reject(new HttpError(403, 'forbidden'))

    expect(await store.fetchModeration()).toBe(false)
    expect(store.moderationForbidden).toBe(true)
    expect(store.error).toBe('')
  })

  it('drops whatever it was holding when access is refused', async () => {
    const { store } = storeWith()
    await store.fetchModeration()
    expect(store.moderationQueue).toHaveLength(1)

    // Whatever is in the list was fetched under an assignment the caller no longer has.
    getMock = () => Promise.reject(new HttpError(403, 'forbidden'))
    await store.fetchModeration()
    expect(store.moderationQueue).toEqual([])
  })

  it('reports a real failure as one', async () => {
    const { store } = storeWith()
    getMock = () => Promise.reject(new HttpError(500, 'boom'))

    expect(await store.fetchModeration()).toBe(false)
    expect(store.moderationForbidden).toBe(false)
    expect(store.error).not.toBe('')
  })

  // Order is the BFF's (task 308): reported-and-not-hidden first, then by count, then newest. The
  // client must not add a second opinion — and must not re-sort after a hide, or the card jumps out
  // from under the moderator's thumb.
  it('preserves the order the BFF sent', async () => {
    const { store } = storeWith()
    getMock = () =>
      Promise.resolve({
        glimt: [
          apiQueueEntry({ id: 'g-reported', report_count: 3, created_at: '2026-09-17T20:00:00Z' }),
          apiQueueEntry({ id: 'g-old', report_count: 0, created_at: '2026-09-17T23:00:00Z' }),
        ],
      })

    await store.fetchModeration()
    expect(store.moderationQueue.map((g) => g.id)).toEqual(['g-reported', 'g-old'])

    await store.setHidden('g-reported', true)
    expect(
      store.moderationQueue.map((g) => g.id),
      'hiding re-ordered the queue under the moderator',
    ).toEqual(['g-reported', 'g-old'])
  })
})

describe('hiding and restoring', () => {
  it('flips the badge optimistically and keeps the card listed', async () => {
    const { store } = storeWith()
    await store.fetchModeration()

    expect(await store.setHidden('g-reported', true)).toBe(true)
    // Still there: a reversal must be possible, and a malicious report is only visible if its
    // target still is.
    expect(store.moderationQueue).toHaveLength(1)
    expect(store.moderationQueue[0].hidden).toBe(true)

    expect(await store.setHidden('g-reported', false)).toBe(true)
    expect(store.moderationQueue[0].hidden).toBe(false)
  })

  // The one lie this screen must not tell.
  it('reverts the flip when the request fails', async () => {
    const { store } = storeWith()
    await store.fetchModeration()
    postMock = () => Promise.reject(new HttpError(500, 'boom'))

    expect(await store.setHidden('g-reported', true)).toBe(false)
    expect(
      store.moderationQueue[0].hidden,
      'a moderator was told a photograph is down while it is still up',
    ).toBe(false)
    expect(store.error).not.toBe('')
  })

  it('updates the cached feed copy too, so the two views cannot contradict each other', async () => {
    const { store } = storeWith()
    await store.fetchModeration()
    store.glimt = [{ ...store.moderationQueue[0] }]

    await store.setHidden('g-reported', true)
    expect(store.glimt[0].hidden).toBe(true)
  })

  it('reverts the cached feed copy on failure as well', async () => {
    const { store } = storeWith()
    await store.fetchModeration()
    store.glimt = [{ ...store.moderationQueue[0] }]
    postMock = () => Promise.reject(new HttpError(500, 'boom'))

    await store.setHidden('g-reported', true)
    expect(store.glimt[0].hidden).toBe(false)
  })

  // The author deleted it while the moderator was looking. Not their failure, and the outcome they
  // wanted has effectively happened — so the row goes rather than sitting there unactionable.
  it('drops a glimt that no longer exists', async () => {
    const { store } = storeWith()
    await store.fetchModeration()
    postMock = () => Promise.reject(new HttpError(404, 'gone'))

    expect(await store.setHidden('g-reported', true)).toBe(false)
    expect(store.moderationQueue).toEqual([])
    expect(store.error).not.toBe('')
  })

  it('records a refusal so the view stops offering the controls', async () => {
    const { store } = storeWith()
    await store.fetchModeration()
    postMock = () => Promise.reject(new HttpError(403, 'forbidden'))

    await store.setHidden('g-reported', true)
    expect(store.moderationForbidden).toBe(true)
    // And the optimistic flip was undone: the glimt is still visible to everyone.
    expect(store.moderationQueue[0].hidden).toBe(false)
  })

  it('calls the hide and unhide endpoints, not a shared one with a body flag', async () => {
    const { store } = storeWith()
    await store.fetchModeration()
    const urls: string[] = []
    postMock = (url: string) => {
      urls.push(url)
      return Promise.resolve(undefined)
    }

    await store.setHidden('g-reported', true)
    await store.setHidden('g-reported', false)
    expect(urls).toEqual([
      '/api/glimt/items/g-reported/hide',
      '/api/glimt/items/g-reported/unhide',
    ])
  })
})

// The structural half: the queue must not be listed in the persisted payload, and the service worker
// must not cache the endpoint. Both are the kind of thing a behavioural test above would miss if
// somebody added a *second* write path.
describe('the moderation payload is not cacheable', () => {
  const STORE_SRC = fileURLToPath(new URL('./glimt.store.ts', import.meta.url))
  const VITE_CONFIG = fileURLToPath(new URL('../../vite.config.ts', import.meta.url))

  it('is absent from the stored payload type and the persist call', () => {
    // Comments stripped first, following `profileNotCached.spec.ts`: the fix for this very hole is
    // documented in a comment that names the thing it prevents, and a guard that fires on its own
    // documentation trains people to delete the documentation.
    const source = readFileSync(STORE_SRC, 'utf8')
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .replace(/^\s*(\/\/|\*).*$/gm, '')

    // The interface that defines what goes to disk.
    const payloadType = source.match(/interface StoredPayload \{[\s\S]*?\n\}/)?.[0] ?? ''
    expect(payloadType, 'StoredPayload not found — did it get renamed?').toContain('glimt')
    expect(payloadType).not.toMatch(/moderation|author/i)

    // And the literal that builds it.
    const persistBody = source.match(/const payload: StoredPayload = \{[\s\S]*?\n {6}\}/)?.[0] ?? ''
    expect(persistBody, 'the persist payload literal was not found').toContain('schema')
    expect(persistBody).not.toMatch(/moderation|author/i)

    // The whitelist that makes the promise hold even when a widened object is assigned to
    // `glimt` — which the type system cannot prevent, because ModerationGlimt extends Glimt.
    expect(persistBody, 'the stored glimt are no longer projected through a whitelist').toContain(
      'toStoredGlimt',
    )
  })

  it('has no service worker caching rule', () => {
    const config = readFileSync(VITE_CONFIG, 'utf8')
    const patterns = config.match(/urlPattern:[\s\S]*?(?=\n\s{12}handler)/g) ?? []
    expect(patterns.length).toBeGreaterThan(0)
    for (const pattern of patterns) {
      expect(pattern).not.toMatch(/moderation/)
    }
  })
})
