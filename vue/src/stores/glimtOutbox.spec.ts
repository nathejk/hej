import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

// The outbox module is mocked at its boundary, not exercised. That follows this repo's existing
// pattern — `trackDb.ts` has no spec either, and `track.store.spec.ts` tests the logic above it —
// and it is the right split here: `glimtOutbox.ts` is IndexedDB plumbing that needs a browser, while
// everything worth getting wrong lives in the store's drain. Adding `fake-indexeddb` would be a new
// dependency to test the part least likely to be subtly wrong.
vi.mock('@/helpers/glimtOutbox', () => ({
  outboxAvailable: () => outboxIsAvailable,
  enqueueGlimt: (draft: unknown, files: unknown) => fake.enqueue(draft, files),
  listGlimtDrafts: () => Promise.resolve(fake.drafts),
  countGlimtDrafts: () => Promise.resolve(fake.drafts.length),
  glimtDraftItems: (id: string) => Promise.resolve(fake.items[id] ?? []),
  markGlimtItemUploaded: (id: string, ordinal: number, uploaded: unknown) =>
    fake.markUploaded(id, ordinal, uploaded),
  recordGlimtDraftFailure: (id: string, message: string) => fake.recordFailure(id, message),
  removeGlimtDraft: (id: string) => fake.remove(id),
}))

vi.mock('@/helpers', async () => {
  const actual = await vi.importActual<typeof import('@/helpers')>('@/helpers')
  return {
    ...actual,
    fetchWrapper: {
      get: vi.fn(),
      post: (url: string, body?: unknown) => calls.post(url, body),
      postForm: (url: string, form: FormData) => calls.postForm(url, form),
      put: vi.fn(),
      putForm: vi.fn(),
      delete: vi.fn(),
    },
  }
})

import { useGlimtStore } from '@/stores/glimt.store'
import { useSessionStore } from '@/stores/session.store'

let outboxIsAvailable = true

// A minimal in-memory stand-in for the outbox, recording what the store asked it to do.
const fake = {
  drafts: [] as Array<{
    id: string
    caption: string
    audience: 'group' | 'nathejk' | 'public'
    createdAt: number
    itemCount: number
    attempts: number
    lastError: string
  }>,
  items: {} as Record<
    string,
    Array<{ draftId: string; ordinal: number; blob: Blob | null; name: string; uploaded: unknown }>
  >,
  enqueued: 0,
  removed: [] as string[],
  failures: [] as Array<{ id: string; message: string }>,
  markedUploaded: [] as Array<{ id: string; ordinal: number }>,

  async enqueue(draft: any, files: any) {
    this.enqueued++
    this.drafts.push({ ...draft, itemCount: files.length, attempts: 0, lastError: '' })
    this.items[draft.id] = files.map((f: any, ordinal: number) => ({
      draftId: draft.id,
      ordinal,
      blob: f.blob,
      name: f.name,
      uploaded: null,
    }))
  },
  async markUploaded(id: string, ordinal: number, uploaded: unknown) {
    this.markedUploaded.push({ id, ordinal })
    const item = (this.items[id] ?? []).find((i) => i.ordinal === ordinal)
    if (item) {
      item.uploaded = uploaded
      // The real store drops the Blob here, and that matters to the resume test.
      item.blob = null
    }
  },
  async recordFailure(id: string, message: string) {
    this.failures.push({ id, message })
    const draft = this.drafts.find((d) => d.id === id)
    if (draft) {
      draft.attempts++
      draft.lastError = message
    }
  },
  async remove(id: string) {
    this.removed.push(id)
    this.drafts = this.drafts.filter((d) => d.id !== id)
    delete this.items[id]
  },
  reset() {
    this.drafts = []
    this.items = {}
    this.enqueued = 0
    this.removed = []
    this.failures = []
    this.markedUploaded = []
  },
}

// What the network did, and what it was asked.
const calls = {
  uploads: [] as string[],
  posts: [] as Array<{ url: string; body: unknown }>,
  failUploadsFrom: -1,
  failCreate: false,

  async postForm(url: string, form: FormData) {
    this.uploads.push(url)
    if (this.failUploadsFrom >= 0 && this.uploads.length > this.failUploadsFrom) {
      throw new Error('upload failed')
    }
    const n = this.uploads.length
    return {
      ref: 'r'.repeat(63) + String(n % 10),
      thumb_ref: 't'.repeat(63) + String(n % 10),
      kind: 'image',
      width: 1600,
      height: 1200,
      bytes: 1000,
    }
  },
  async post(url: string, body?: unknown) {
    this.posts.push({ url, body })
    if (this.failCreate) throw new Error('create failed')
    return {
      id: 'g-created',
      hold: { number: '42', name: 'Ørnene', group: 'spejder' },
      own: true,
      audience: 'group',
      caption: '',
      created_at: new Date().toISOString(),
      media: [],
      hidden: false,
    }
  },
  reset() {
    this.uploads = []
    this.posts = []
    this.failUploadsFrom = -1
    this.failCreate = false
  },
}

function file(name = 'a.jpg') {
  return { blob: new Blob(['bytes'], { type: 'image/jpeg' }), name }
}

beforeEach(() => {
  setActivePinia(createPinia())
  useSessionStore().user = { userId: 'p-viewer', role: 'spejder' }
  outboxIsAvailable = true
  fake.reset()
  calls.reset()
  vi.stubGlobal('crypto', { randomUUID: () => `draft-${fake.enqueued + 1}` })
})

describe('queue', () => {
  // The promise in one test: the files are persisted *before* anything is attempted, so a post
  // survives a failed upload, a locked phone, and an app the OS killed.
  it('writes to the outbox before trying to send', async () => {
    calls.failUploadsFrom = 0 // every upload fails
    const store = useGlimtStore()

    const result = await store.queue({
      caption: 'hej',
      audience: 'group',
      files: [file()],
    })

    expect(result.queued).toBe(true)
    expect(result.sent).toBe(false)
    expect(fake.enqueued).toBe(1)
    // Still waiting, not lost.
    expect(fake.drafts).toHaveLength(1)
    expect(store.pending).toBe(1)
  })

  it('sends immediately when it can', async () => {
    const store = useGlimtStore()
    const result = await store.queue({ caption: 'hej', audience: 'group', files: [file()] })

    expect(result).toEqual({ queued: true, sent: true })
    expect(calls.uploads).toHaveLength(1)
    expect(calls.posts).toHaveLength(1)
    // Gone from the outbox once it landed.
    expect(fake.removed).toEqual(['draft-1'])
    expect(store.pending).toBe(0)
  })

  it('prepends the created glimt so the author sees their own post at once', async () => {
    const store = useGlimtStore()
    await store.queue({ caption: 'hej', audience: 'group', files: [file()] })
    expect(store.glimt.map((g) => g.id)).toEqual(['g-created'])
  })

  // No IndexedDB — a browser that has blocked it. Sending directly is better than refusing to post,
  // but the caller is told it was not queued so it can say plainly that this one will not survive.
  it('falls back to sending directly with no outbox', async () => {
    outboxIsAvailable = false
    const store = useGlimtStore()

    const result = await store.queue({ caption: 'hej', audience: 'group', files: [file()] })
    expect(result).toEqual({ queued: false, sent: true })
    expect(fake.enqueued).toBe(0)
    expect(calls.posts).toHaveLength(1)
  })
})

describe('drain', () => {
  async function queueDraft(id: string, createdAt: number, files = [file()]) {
    await fake.enqueue({ id, caption: id, audience: 'group', createdAt }, files)
  }

  it('sends oldest first', async () => {
    await queueDraft('draft-old', 1000)
    await queueDraft('draft-new', 2000)
    const store = useGlimtStore()

    await store.drain()

    expect(fake.removed).toEqual(['draft-old', 'draft-new'])
  })

  // Continuing past a failure would burn a data budget re-failing on the same dead connection, and
  // the queue is ordered because the member posted in an order.
  it('stops at the first draft that fails', async () => {
    await queueDraft('draft-1', 1000)
    await queueDraft('draft-2', 2000)
    calls.failCreate = true
    const store = useGlimtStore()

    await store.drain()

    expect(fake.removed).toEqual([])
    // Only the first draft was attempted: one upload, not two.
    expect(calls.uploads).toHaveLength(1)
    expect(fake.failures.map((f) => f.id)).toEqual(['draft-1'])
  })

  // The reason the outbox stores refs per item. On rural mobile data this is the difference between a
  // post that eventually lands and one that never does.
  it('resumes a part-uploaded draft instead of starting over', async () => {
    await queueDraft('draft-1', 1000, [file('a.jpg'), file('b.jpg'), file('c.jpg')])
    // The first two already landed on an earlier attempt, and their Blobs were dropped.
    await fake.markUploaded('draft-1', 0, { ref: 'a', thumbRef: '', kind: 'image', width: 1, height: 1, bytes: 1, durationMs: 0 })
    await fake.markUploaded('draft-1', 1, { ref: 'b', thumbRef: '', kind: 'image', width: 1, height: 1, bytes: 1, durationMs: 0 })
    fake.markedUploaded = []
    const store = useGlimtStore()

    await store.drain()

    // Only the third item was uploaded.
    expect(calls.uploads).toHaveLength(1)
    expect(fake.markedUploaded).toEqual([{ id: 'draft-1', ordinal: 2 }])
    // And all three refs reached the create call, in order.
    const body = calls.posts[0].body as { media: Array<{ ref: string }> }
    expect(body.media.map((m) => m.ref)).toEqual(['a', 'b', expect.stringContaining('r')])
  })

  it('records the failure on the draft so the UI can say something useful', async () => {
    await queueDraft('draft-1', 1000)
    calls.failCreate = true
    const store = useGlimtStore()

    await store.drain()

    expect(fake.drafts[0].attempts).toBe(1)
    expect(fake.drafts[0].lastError).not.toBe('')
  })

  // Foreground and `online` fire together often enough — unlock a phone in a coverage hole and both
  // arrive within a second. Two overlapping drains would upload everything twice.
  it('does not overlap', async () => {
    await queueDraft('draft-1', 1000)
    const store = useGlimtStore()

    await Promise.all([store.drain(), store.drain()])

    expect(calls.uploads).toHaveLength(1)
    expect(fake.removed).toEqual(['draft-1'])
  })

  it('does nothing with no outbox', async () => {
    outboxIsAvailable = false
    await queueDraft('draft-1', 1000)
    const store = useGlimtStore()

    expect(await store.drain()).toBe(false)
    expect(calls.uploads).toHaveLength(0)
  })

  // A draft whose items have neither bytes nor refs can never be posted. Removing it is right;
  // retrying it forever would block every post behind it, since the drain stops at the first failure.
  it('removes a draft with nothing left to send', async () => {
    await fake.enqueue({ id: 'draft-empty', caption: '', audience: 'group', createdAt: 1 }, [])
    const store = useGlimtStore()

    await store.drain()

    expect(fake.removed).toEqual(['draft-empty'])
    expect(calls.posts).toHaveLength(0)
  })

  // A queued post that has not gone yet is a normal state on this network. An error banner on every
  // foreground would train people to ignore it.
  it('does not surface a failed drain as an error banner', async () => {
    await queueDraft('draft-1', 1000)
    calls.failCreate = true
    const store = useGlimtStore()

    await store.drain()

    expect(store.error).toBe('')
  })
})

describe('discardPending', () => {
  // The member's decision, and the only way a draft leaves the outbox unsent. Nothing gives up on a
  // post by itself — that is the promise.
  it('removes a draft and updates the count', async () => {
    await fake.enqueue({ id: 'draft-1', caption: '', audience: 'group', createdAt: 1 }, [file()])
    const store = useGlimtStore()
    await store.refreshPending()
    expect(store.pending).toBe(1)

    await store.discardPending('draft-1')

    expect(fake.removed).toEqual(['draft-1'])
    expect(store.pending).toBe(0)
  })
})
