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

import { requestPersistence } from '@/helpers/offline/persistence'

const DB_NAME = 'hej-track'
const DB_VERSION = 2
const STORE = 'points'
const EVENTS = 'events'

/**
 * How many diagnostic events are kept.
 *
 * A ring, not a log: this exists to answer "what did the recorder do on a real phone
 * overnight" (task 082's device measurement), and unbounded growth would compete for the
 * quota with the thing being measured.
 *
 * Sized against that measurement rather than picked round. A 12-hour test at 30 s
 * sampling produces ~1,440 `point` events on its own, so a smaller ring would spend
 * itself on routine successes and discard precisely the `hidden`/`visible` transitions
 * the test is for. 2,000 covers a full night with room left; at roughly 60 bytes an entry
 * that is ~120 KB, which is nothing beside the 195 KB of points it annotates.
 */
const MAX_EVENTS = 2000

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
/**
 * One diagnostic event.
 *
 * Persisted rather than kept in memory precisely because the interesting cases destroy
 * memory: iOS suspending a backgrounded app, and the OS killing it outright. An event
 * written before the app went away is the only evidence that survives to be read
 * afterwards.
 */
export interface TrackEvent {
  at: number
  kind:
    | 'load'
    | 'start'
    | 'stop'
    | 'point'
    | 'skip'
    | 'nofix'
    | 'geoerror'
    | 'full'
    | 'capped'
    | 'nostart'
    | 'upload'
    | 'uploadfail'
    | 'hidden'
    | 'visible'
    // Not the track's own, but this is the app's only diagnostic channel that survives being killed, and
    // a failed offline sync is exactly the kind of thing that needs to be readable after the fact
    // (task 203). Kept here rather than given a second log nobody would think to look at.
    | 'syncfail'
  detail?: string
}

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
      // Added in v2. Guarded by the same `contains` check as the points store so the
      // upgrade is correct from either a fresh install or an existing v1 database.
      if (!db.objectStoreNames.contains(EVENTS)) {
        db.createObjectStore(EVENTS, { keyPath: 'seq', autoIncrement: true })
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
 * pendingPoints returns up to `limit` of a user's not-yet-uploaded points, oldest first.
 *
 * Oldest first matters: a member who has been offline for hours should ship the beginning of
 * their track first, so a partially successful backlog leaves a contiguous route rather than
 * a scattering of the most recent fixes.
 */
export async function pendingPoints(userId: string, limit: number): Promise<TrackPoint[]> {
  const conn = await db()
  return new Promise((resolve, reject) => {
    const index = conn.transaction(STORE, 'readonly').objectStore(STORE).index('pending')
    // The index is on [userId, uploaded], so this range is exactly "this user's pending
    // points" — the database does the filtering rather than the caller reading everything
    // and discarding most of it.
    const req = index.openCursor(IDBKeyRange.only([userId, 0]))
    const out: TrackPoint[] = []
    req.onsuccess = () => {
      const cursor = req.result
      if (!cursor || out.length >= limit) return resolve(out)
      out.push(cursor.value as TrackPoint)
      cursor.continue()
    }
    req.onerror = () => reject(req.error)
  })
}

/** countPending returns how many of a user's points have not been uploaded. */
export async function countPending(userId: string): Promise<number> {
  const conn = await db()
  return new Promise((resolve, reject) => {
    const req = conn
      .transaction(STORE, 'readonly')
      .objectStore(STORE)
      .index('pending')
      .count(IDBKeyRange.only([userId, 0]))
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

/**
 * markUploaded flags points as accepted by the server.
 *
 * Marking rather than deleting, and only after a 2xx: a point is the only copy that exists
 * until the server has it (PRD 002 §11.1), so the flag moves exactly once, in the one place
 * that knows the server accepted it.
 *
 * Writes each point back under its own key in one transaction, so either all of the batch is
 * marked or none is. A partial mark would be the worst outcome — it would leave points that
 * the server has but the client still thinks are pending, and re-upload them forever.
 */
export async function markUploaded(userId: string, timestamps: number[]): Promise<void> {
  if (timestamps.length === 0) return
  const conn = await db()
  return new Promise((resolve, reject) => {
    const tx = conn.transaction(STORE, 'readwrite')
    const store = tx.objectStore(STORE)
    for (const ts of timestamps) {
      const get = store.get([userId, ts])
      get.onsuccess = () => {
        const point = get.result as TrackPoint | undefined
        // Absent is not an error: a concurrent prune may have removed it, and re-adding
        // it here would resurrect a point the device deliberately dropped.
        if (point) store.put({ ...point, uploaded: 1 })
      }
    }
    tx.oncomplete = () => resolve()
    tx.onabort = tx.onerror = () => reject(tx.error ?? new Error('mark uploaded failed'))
  })
}

/**
 * pruneUploaded deletes a user's already-uploaded points older than `before`.
 *
 * Uploaded points are kept for a while rather than deleted on acceptance, so the status page
 * can still say what this device recorded during the event — "9 points, 2 pending" is a
 * useful answer and "2 points" is a misleading one. They are not kept forever: the quota is
 * shared with map tiles and portraits, so anything the server already has stops earning its
 * space after the race it belongs to.
 */
export async function pruneUploaded(userId: string, before: number): Promise<number> {
  const conn = await db()
  return new Promise((resolve, reject) => {
    const tx = conn.transaction(STORE, 'readwrite')
    const index = tx.objectStore(STORE).index('pending')
    const req = index.openCursor(IDBKeyRange.only([userId, 1]))
    let deleted = 0
    req.onsuccess = () => {
      const cursor = req.result
      if (!cursor) return
      if ((cursor.value as TrackPoint).ts < before) {
        cursor.delete()
        deleted++
      }
      cursor.continue()
    }
    tx.oncomplete = () => resolve(deleted)
    tx.onabort = tx.onerror = () => reject(tx.error ?? new Error('prune failed'))
  })
}

/**
 * logEvent appends a diagnostic event, trimming the ring when it grows past the cap.
 *
 * Deliberately never throws: this is instrumentation, and instrumentation that can break
 * the thing it observes is worse than none. A failure here is swallowed, including a
 * quota failure — the points are what matter, and appendPoint reports that properly.
 */
export async function logEvent(kind: TrackEvent['kind'], detail?: string): Promise<void> {
  try {
    const conn = await db()
    await new Promise<void>((resolve) => {
      const tx = conn.transaction(EVENTS, 'readwrite')
      const store = tx.objectStore(EVENTS)
      store.add({ at: Date.now(), kind, ...(detail ? { detail } : {}) })
      // Trim from the front. autoIncrement keys are monotonic, so "oldest" is simply
      // the lowest key.
      const countReq = store.count()
      countReq.onsuccess = () => {
        const excess = countReq.result - MAX_EVENTS
        if (excess > 0) {
          const cursorReq = store.openCursor()
          let removed = 0
          cursorReq.onsuccess = () => {
            const cursor = cursorReq.result
            if (cursor && removed < excess) {
              cursor.delete()
              removed += 1
              cursor.continue()
            }
          }
        }
      }
      tx.oncomplete = () => resolve()
      tx.onabort = tx.onerror = () => resolve()
    })
  } catch {
    // See above: instrumentation must not be able to break recording.
  }
}

/** listEvents returns the diagnostic events, oldest first. */
export async function listEvents(): Promise<TrackEvent[]> {
  const conn = await db()
  return new Promise((resolve, reject) => {
    const req = conn.transaction(EVENTS, 'readonly').objectStore(EVENTS).getAll()
    req.onsuccess = () => resolve(req.result as TrackEvent[])
    req.onerror = () => reject(req.error)
  })
}

/** listPoints returns every stored point, for the status report's gap analysis. */
export async function listPoints(): Promise<TrackPoint[]> {
  const conn = await db()
  return new Promise((resolve, reject) => {
    const req = conn.transaction(STORE, 'readonly').objectStore(STORE).getAll()
    req.onsuccess = () => resolve(req.result as TrackPoint[])
    req.onerror = () => reject(req.error)
  })
}

/**
 * requestPersistentStorage asks the browser not to evict this origin's data, and
 * reports what it decided.
 *
 * **Delegates to `helpers/offline/persistence.ts` (task 185).** The request is
 * per-origin — one answer covers the track, the map tiles, the contacts directory and
 * the app shell — so it no longer belongs to this file. Kept as a named function here
 * because the track has a specific reason to care that the others do not: an evicted
 * track cannot be re-fetched from anywhere, while tiles and portraits can.
 *
 * Returns a boolean rather than the three-valued outcome because that is all the track
 * status page shows; `offline.store` keeps the distinction between "denied" and "this
 * browser does not do persistence".
 */
export async function requestPersistentStorage(): Promise<boolean> {
  if (typeof navigator === 'undefined') return false
  return (await requestPersistence(navigator.storage)) === 'granted'
}
