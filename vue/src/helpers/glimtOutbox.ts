// The Glimt outbox: drafts and their media, kept until they reach the server (PRD 019 §5, task 314).
//
// # The promise this exists to keep
//
// "The app must not lose a photo it accepted" (PRD 019 §3). A spejder posts from a field at night on
// one bar of signal; the composer accepts the glimt, and then the upload fails, or the phone locks, or
// iOS kills the tab. Without this file all three lose the photographs — and the member has already
// moved on and cannot retake them.
//
// # Why IndexedDB, not localStorage
//
// The same reason `trackDb.ts` gives, and more so: this holds **Blobs**. localStorage is
// string-only and ~5 MB, so a single glimt of four phone photographs would not fit even
// base64-encoded, and encoding them would inflate them by a third on the way in. IndexedDB stores a
// Blob natively.
//
// Raw IndexedDB rather than a wrapper, as `trackDb.ts` argues: two object stores, a handful of
// operations, no schema evolution yet. A dependency would be more code than this file.
//
// # What is *not* here: Background Sync
//
// There is no `sync.register`, and there must not be. Background Sync is unavailable on iOS, and a
// backgrounded web app does not run there at all — PRD 002 measured 2% coverage over a 22-hour day.
// So the drain is triggered by foreground and `online` only (see `drainGlimtOutbox`'s callers), and
// **the UI must never imply an upload is happening while the app is closed**. A pending glimt says
// "venter på nettet", not "sender i baggrunden".

const DB_NAME = 'hej-glimt'
const DB_VERSION = 1
/** Drafts: one row per unsent glimt. */
const DRAFTS = 'drafts'
/** Media: one row per file, keyed by draft and position. Separate so a Blob is never rewritten. */
const MEDIA = 'media'

/**
 * A queued glimt.
 *
 * `id` is generated on the client, so a draft has an identity before the server has seen it — which is
 * what lets the composer show it, the drain find it again after a restart, and a retry be recognised
 * as the same post rather than a second one.
 */
export interface GlimtDraft {
  id: string
  caption: string
  audience: 'group' | 'nathejk' | 'public'
  /** Epoch ms, for ordering the queue and for showing how long something has waited. */
  createdAt: number
  /** How many items belong to this draft. Stored so the queue can be described without reading Blobs. */
  itemCount: number
  /**
   * How many drain attempts have failed.
   *
   * Kept so the UI can stop saying "prøver igen" forever and start saying something a member can act
   * on. Deliberately not used to *give up*: the whole point is that a post survives until it lands or
   * the member removes it, and a phone that has been in a pocket for six hours has failed a lot of
   * attempts through no fault of the post.
   */
  attempts: number
  /** The last failure, for the UI. Empty when nothing has gone wrong yet. */
  lastError: string
}

/**
 * One queued file, plus the refs once it has been uploaded.
 *
 * The `uploaded` refs are the reason this store exists separately from the draft: a drain that fails
 * on item four must not re-upload items one to three. After a successful upload the Blob is *dropped*
 * and the refs kept, so a half-uploaded draft stops costing quota for bytes the server already has.
 */
export interface GlimtDraftItem {
  draftId: string
  ordinal: number
  /** The file, until it is uploaded. Null afterwards. */
  blob: Blob | null
  /** Original filename, for the multipart part. */
  name: string
  /** The server's refs, once uploaded. Null until then. */
  uploaded: {
    ref: string
    thumbRef: string
    kind: 'image' | 'video'
    width: number
    height: number
    bytes: number
    durationMs: number
  } | null
}

function open(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const database = req.result
      if (!database.objectStoreNames.contains(DRAFTS)) {
        database.createObjectStore(DRAFTS, { keyPath: 'id' })
      }
      if (!database.objectStoreNames.contains(MEDIA)) {
        // Keyed by [draftId, ordinal], which makes the author's arrangement a constraint the
        // database enforces rather than a convention the drain has to remember — and makes a
        // re-put of the same item an overwrite instead of a duplicate.
        database.createObjectStore(MEDIA, { keyPath: ['draftId', 'ordinal'] })
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

// Lazily opened and reused. Not cached on failure: private-mode Safari and a full disk both fail here,
// and both can recover.
let handle: Promise<IDBDatabase> | null = null

function db(): Promise<IDBDatabase> {
  if (!handle) {
    handle = open().catch((err) => {
      handle = null
      throw err
    })
  }
  return handle
}

/**
 * Whether the platform can queue at all.
 *
 * False in a node test run and in a browser that has blocked IndexedDB. The composer must check: with
 * no outbox, an offline post genuinely cannot be kept, and telling a member it was queued would be the
 * one lie this file exists to prevent.
 */
export function outboxAvailable(): boolean {
  try {
    return typeof indexedDB !== 'undefined'
  } catch {
    // Access alone throws in some privacy modes.
    return false
  }
}

function tx(database: IDBDatabase, stores: string[], mode: IDBTransactionMode) {
  return database.transaction(stores, mode)
}

function done(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error)
    transaction.onabort = () => reject(transaction.error)
  })
}

function asPromise<T>(req: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

/**
 * Queue a draft and its files in one transaction.
 *
 * One transaction on purpose: a draft with no media is a card that can never be posted, and media with
 * no draft is quota nobody will ever reclaim. Either both land or neither does.
 */
export async function enqueueGlimt(
  draft: Omit<GlimtDraft, 'attempts' | 'lastError' | 'itemCount'>,
  files: Array<{ blob: Blob; name: string }>,
): Promise<void> {
  const database = await db()
  const transaction = tx(database, [DRAFTS, MEDIA], 'readwrite')

  const row: GlimtDraft = {
    ...draft,
    itemCount: files.length,
    attempts: 0,
    lastError: '',
  }
  transaction.objectStore(DRAFTS).put(row)

  const media = transaction.objectStore(MEDIA)
  files.forEach((file, ordinal) => {
    const item: GlimtDraftItem = {
      draftId: draft.id,
      ordinal,
      blob: file.blob,
      name: file.name,
      uploaded: null,
    }
    media.put(item)
  })

  await done(transaction)
}

/** Every queued draft, oldest first — the order they should be sent in. */
export async function listGlimtDrafts(): Promise<GlimtDraft[]> {
  const database = await db()
  const drafts = await asPromise(
    tx(database, [DRAFTS], 'readonly').objectStore(DRAFTS).getAll() as IDBRequest<GlimtDraft[]>,
  )
  return drafts.sort((a, b) => a.createdAt - b.createdAt)
}

/** How many drafts are waiting. Cheap enough to call on every render. */
export async function countGlimtDrafts(): Promise<number> {
  if (!outboxAvailable()) return 0
  try {
    const database = await db()
    return await asPromise(tx(database, [DRAFTS], 'readonly').objectStore(DRAFTS).count())
  } catch {
    // A count is decoration; it must not break a view.
    return 0
  }
}

/** One draft's items, in the author's order. */
export async function glimtDraftItems(draftId: string): Promise<GlimtDraftItem[]> {
  const database = await db()
  const all = await asPromise(
    tx(database, [MEDIA], 'readonly').objectStore(MEDIA).getAll() as IDBRequest<GlimtDraftItem[]>,
  )
  return all.filter((i) => i.draftId === draftId).sort((a, b) => a.ordinal - b.ordinal)
}

/**
 * Record that one item has been uploaded, and **drop its Blob**.
 *
 * Dropping the bytes is the point. A draft that failed on its last item would otherwise keep every
 * earlier photograph on disk for as long as it waits — which on a phone that is already short of space
 * is how the outbox becomes the reason the next glimt cannot be queued.
 */
export async function markGlimtItemUploaded(
  draftId: string,
  ordinal: number,
  uploaded: NonNullable<GlimtDraftItem['uploaded']>,
): Promise<void> {
  const database = await db()
  const transaction = tx(database, [MEDIA], 'readwrite')
  const store = transaction.objectStore(MEDIA)
  const existing = await asPromise(store.get([draftId, ordinal]) as IDBRequest<GlimtDraftItem>)
  if (existing) {
    store.put({ ...existing, blob: null, uploaded })
  }
  await done(transaction)
}

/** Record a failed attempt, so the UI can say something more useful than "venter". */
export async function recordGlimtDraftFailure(draftId: string, message: string): Promise<void> {
  const database = await db()
  const transaction = tx(database, [DRAFTS], 'readwrite')
  const store = transaction.objectStore(DRAFTS)
  const existing = await asPromise(store.get(draftId) as IDBRequest<GlimtDraft>)
  if (existing) {
    store.put({ ...existing, attempts: existing.attempts + 1, lastError: message })
  }
  await done(transaction)
}

/**
 * Remove a draft and everything belonging to it.
 *
 * Called on success and when a member discards a post. One transaction across both stores, so a
 * removed draft cannot leave orphaned Blobs behind — the leak nobody would ever notice, because
 * nothing lists them.
 */
export async function removeGlimtDraft(draftId: string): Promise<void> {
  const database = await db()
  const transaction = tx(database, [DRAFTS, MEDIA], 'readwrite')
  transaction.objectStore(DRAFTS).delete(draftId)

  const media = transaction.objectStore(MEDIA)
  const all = await asPromise(media.getAll() as IDBRequest<GlimtDraftItem[]>)
  for (const item of all) {
    if (item.draftId === draftId) media.delete([item.draftId, item.ordinal])
  }

  await done(transaction)
}
