// What race area the cached map tiles were downloaded for (task 294, PRD 017).
//
// # Why this exists, and why it is not a refresh
//
// The race area is the one sync dataset with **nothing cached to refresh**: it is fetched on demand when
// a bulk tile download starts, and no copy is kept. So a changed `race_area` version cannot be handled
// like the other five — there is no payload to replace. What it actually means is a *user-visible fact*:
// the tiles on this device were downloaded for a smaller, or differently-shaped, area than the event now
// has, so there is map the user believes they have offline and does not.
//
// That matters at exactly the moment it cannot be fixed — out of signal, at night, at the edge of the
// area. Hence a notice on the readiness screen rather than a silent re-download: a few hundred megabytes
// stays a decision the user makes, which is that screen's existing contract.
//
// # Device-wide, not per profile
//
// Unlike the contacts directory or a patrol's map sheets, this is not personal data and not
// patrol-scoped: every device in the event shares one race area. So the key is device-wide on purpose. A
// sibling switching profiles on a shared phone must not be told to re-download 324 MB of tiles that are
// already there and already correct.

const STORAGE_KEY = 'hej.tiles.areaVersion.v1'

/**
 * The browser storage this needs, as an argument rather than a global — same seam as the stores.
 *
 * `localStorage` is absent in a node test run and *throws on access* in some Safari privacy modes, so
 * even reading the global needs a guard.
 */
export interface VersionStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

function browserStorage(): VersionStorage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}

/** Records the version of the area a completed tile download covered. */
export function rememberTileAreaVersion(version: string, storage = browserStorage()) {
  if (!storage || !version) return
  try {
    storage.setItem(STORAGE_KEY, version)
  } catch {
    // Quota, or a privacy mode. The tiles are still downloaded and still work; all that is lost is the
    // ability to notice a later change, which is not worth failing a 324 MB download over.
  }
}

/** The version the cached tiles were downloaded for, or '' when unknown. */
export function heldTileAreaVersion(storage = browserStorage()): string {
  if (!storage) return ''
  try {
    return storage.getItem(STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

export function forgetTileAreaVersion(storage = browserStorage()) {
  if (!storage) return
  try {
    storage.removeItem(STORAGE_KEY)
  } catch {
    // Nothing to do, and nothing depends on it: an orphaned version only ever produces a notice that
    // task 294's own rule suppresses anyway (no tiles held → nothing to be stale).
  }
}

/**
 * Whether the event's area has moved beyond what this device downloaded.
 *
 * Two rules, both of which exist to avoid nagging somebody about nothing:
 *
 *   - **No held version means no claim.** A device that has never downloaded tiles has nothing to be
 *     stale, and a device that downloaded them before this version existed (task 294 shipped after
 *     task 087) must not be told its map is out of date on the strength of a value we never stored.
 *     Absent is not stale.
 *   - **No served version means no claim** either. The sync check omits `race_area` when the caller
 *     cannot hold it, and lists it as unavailable when the server could not derive it; neither is
 *     evidence that anything moved.
 */
export function tileAreaIsStale(held: string, served: string): boolean {
  if (held === '' || served === '') return false
  return held !== served
}
