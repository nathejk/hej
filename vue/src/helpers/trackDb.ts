// Local persistent storage for the position track (PRD 002 §11.1, task 082).
//
// WHY INDEXEDDB, NOT localStorage. An unshipped track is the only data in this app
// that cannot be recovered from the server — portraits share that property and are
// treated with the same care. localStorage is synchronous, string-only and ~5 MB;
// blocking the main thread on every position fix is a poor trade on a phone, and 5 MB
// is not a budget worth reasoning about when the alternative is free.
//
// Raw IndexedDB rather than a wrapper library: this needs one object store, three
// operations and no schema evolution yet. A dependency would be more code than this
// file, not less.

const DB_NAME = 'hej-track'
const DB_VERSION = 1
const STORE = 'points'

/** A recorded position. */
export interface TrackPoint {
  /** Who was carrying the phone. See the note on the key path below. */
  userId: string
  /** Epoch milliseconds. Also half the primary key. */
  ts: number
  lat: number
  lng: number
  /** Metres, as reported by the browser. Kept because a 2 km fix is not a position. */
  accuracy: number
  /** 0 = not yet accepted by the server, 1 = accepted. Numbers because IndexedDB
   *  cannot index booleans. Written by task 083; only ever 0 here. */
  uploaded: 0 | 1
}

// The key is [userId, ts], which is PRD 002 §11.1's "(person, timestamp) identifies a
// point" expressed as a constraint the database enforces rather than a convention the
// upload code has to remember. Two consequences worth stating:
//
//   * Re-recording the same instant OVERWRITES instead of appending, so a retry, a
//     double-started recorder or a clock that steps backwards cannot produce
//     duplicates. Task 083's "a retried batch does not duplicate points" is therefore
//     true of the store itself, not just of the upload protocol.
//   * The track is per person, not per device. That matters here: roughly 1 in 8
//     numbers is shared by siblings (task 079), so one phone can legitimately carry
//     two people's tracks. Keying by device would silently merge them into one route
//     through the forest that nobody walked.
function open(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(STORE)) {
        const store = db.createObjectStore(STORE, { keyPath: ['userId', 'ts'] })
        // Task 083 selects "this person's points that have not shipped".
        store.createIndex('pending', ['userId', 'uploaded'])
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

// Lazily opened and reused: opening per write would serialise every fix behind a
// connection handshake.
let handle: Promise<IDBDatabase> | null = null

function db(): Promise<IDBDatabase> {
  if (!handle) {
    handle = open().catch((err) => {
      // Don't cache a failed open — private-mode Safari and a full disk both fail
      // here, and both can recover.
      handle = null
      throw err
    })
  }
  return handle
}

/** Thrown when the browser refused the write because storage is full. */
export class TrackStorageFullError extends Error {
  constructor(cause?: unknown) {
    super('Der er ikke plads til flere sporingspunkter på telefonen.')
    this.name = 'TrackStorageFullError'
    this.cause = cause
  }
}

function isQuotaError(err: unknown): boolean {
  // QuotaExceededError is a DOMException; the name is the only reliable signal
  // across engines. Firefox has historically also used the legacy code 22.
  return err instanceof DOMException && (err.name === 'QuotaExceededError' || err.code === 22)
}

/**
 * appendPoint stores one position.
 *
 * Rejects with TrackStorageFullError when the quota is exhausted, so the caller can
 * stop recording and say so, rather than letting an unhandled rejection disappear —
 * silently failing writes would look exactly like a working recorder.
 */
export async function appendPoint(point: TrackPoint): Promise<void> {
  const conn = await db()
  return new Promise((resolve, reject) => {
    const tx = conn.transaction(STORE, 'readwrite')
    tx.objectStore(STORE).put(point)
    tx.oncomplete = () => resolve()
    tx.onabort = tx.onerror = () => {
      const err = tx.error
      reject(isQuotaError(err) ? new TrackStorageFullError(err) : (err ?? new Error('track write failed')))
    }
  })
}

/**
 * latestTimestamp returns the newest recorded timestamp for a user, or 0.
 *
 * Needed because the recorder's "have I sampled recently?" question must survive a page
 * load: every full navigation remounts the app and would otherwise take an immediate
 * fix, so a few reloads produce a burst of points seconds apart instead of the 30 s
 * cadence. The answer has to come from the database, not from process memory.
 *
 * Uses the compound key's ordering: keys are [userId, ts], so opening a cursor over the
 * user's key range in reverse yields their newest point first, without scanning.
 *
 * The bounds are plain numbers on purpose. `-Infinity`/`Infinity` are NOT valid
 * IndexedDB keys and make `IDBKeyRange.bound` throw a DataError — which, in the version
 * of this that shipped to the test harness, was swallowed into a console error, left
 * lastPointAt at 0, and made every page load take an immediate fix. 8.64e15 ms is the
 * maximum value of a JavaScript Date, so no real timestamp can exceed it.
 */
export async function latestTimestamp(userId: string): Promise<number> {
  const conn = await db()
  return new Promise((resolve, reject) => {
    const range = IDBKeyRange.bound([userId, 0], [userId, 8.64e15])
    const req = conn.transaction(STORE, 'readonly').objectStore(STORE).openCursor(range, 'prev')
    req.onsuccess = () => {
      const cursor = req.result
      resolve(cursor ? (cursor.value as TrackPoint).ts : 0)
    }
    req.onerror = () => reject(req.error)
  })
}

/** countPoints returns how many points are stored, for all users on this device. */
export async function countPoints(): Promise<number> {
  const conn = await db()
  return new Promise((resolve, reject) => {
    const req = conn.transaction(STORE, 'readonly').objectStore(STORE).count()
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

/**
 * requestPersistentStorage asks the browser not to evict this origin's data, and
 * reports what it decided.
 *
 * Worth calling rather than decorative: WebKit grants persistence "based on
 * heuristics like whether the website is opened as a Home Screen Web App", which is
 * exactly this app's install-first onboarding. Without it the track sits in
 * best-effort storage and can be evicted under pressure — and unlike map tiles, an
 * evicted track cannot be re-fetched from anywhere.
 *
 * Returns the outcome instead of assuming it, so the answer can be shown to the user
 * (see PrivacyView) rather than hoped for.
 */
export async function requestPersistentStorage(): Promise<boolean> {
  if (typeof navigator === 'undefined' || !navigator.storage?.persist) return false
  try {
    if (await navigator.storage.persisted()) return true
    return await navigator.storage.persist()
  } catch {
    return false
  }
}
