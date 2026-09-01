// Asking the browser not to evict this origin's data (PRD 009 §6, task 185).
//
// # Why this is app-level and not per-feature
//
// `navigator.storage.persist()` is a **per-origin** request: one answer covers the map tiles,
// the contacts directory, the app shell and the position track together. It first shipped inside
// `trackDb` because the track needed it first, which meant the answer depended on whichever
// feature happened to run earliest. This module is the one caller; `trackDb` now delegates here.
//
// # Why it is worth asking at all
//
// Everything a browser stores is best-effort and evictable by default. WebKit grants persistence
// "based on heuristics like whether the website is opened as a Home Screen Web App" — which is
// exactly what PRD 005's install-first onboarding produces, so this request should actually
// succeed rather than being a formality. The same install decision also exempts the app from
// Safari's seven-day inactivity eviction.
//
// It remains a *request*. Even when granted, quota is an upper bound with no guarantee, so
// `QuotaExceededError` is still expected (task 186) and the readiness view still has to be able
// to say the data is gone.

/**
 * What the browser said.
 *
 * Three outcomes rather than a boolean, because they call for three different sentences in the
 * readiness view — and only one of them ("denied") is worth telling the user about at all.
 * Collapsing "denied" into "unsupported" would have us warn people on browsers that were never
 * going to answer.
 */
export type PersistenceOutcome = 'granted' | 'denied' | 'unsupported'

/** The subset of `navigator.storage` used here. Injected, so the spec can run in node. */
export interface PersistenceStorageManager {
  persist?: () => Promise<boolean>
  persisted?: () => Promise<boolean>
}

/**
 * Ask for persistent storage, unless it has already been granted.
 *
 * `persisted()` is checked first because a repeat `persist()` is not free: on some engines it can
 * surface a prompt, and asking a user who already said yes is a good way to have them say no.
 * That check is also what makes this safe to call from more than one place.
 */
export async function requestPersistence(
  storage: PersistenceStorageManager | undefined,
): Promise<PersistenceOutcome> {
  if (!storage?.persist) return 'unsupported'
  try {
    if (storage.persisted && (await storage.persisted())) return 'granted'
    return (await storage.persist()) ? 'granted' : 'denied'
  } catch {
    // Private-mode browsers throw here rather than returning false. Reporting 'unsupported'
    // instead of 'denied' keeps us from warning a user about a decision nobody made.
    return 'unsupported'
  }
}
