// Per-profile storage keys (PRD 012 §8, task 180).
//
// Several profiles can share one phone number and therefore one device — 213 numbers in the real
// data, and usually the *same person* with duplicate registrations (PRD 006 §11 Q1). Anything the
// app caches about "who I can see" is therefore per profile, not per device.
//
// This was already wrong before the profile switcher existed: sign out, sign in as a sibling, and
// the previous profile's cached directory and favourites were what you saw. Rare enough to go
// unnoticed; the switcher would have made it routine, on a cache holding colleagues' names, numbers
// and portrait references.
//
// **Keying rather than clearing on switch**, deliberately: a key that includes the profile holds
// even if some future code path forgets to clear, and switching *back* finds that profile's own
// cache intact rather than an empty pane — which matters for the parent-with-two-children case,
// where switching is frequent.

/** The subset of `Storage` these helpers need; see the seam note in `contacts.store`. */
export interface ScopedStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

/**
 * The storage key for `base` belonging to `userId`.
 *
 * Returns null without a user id, and callers must treat that as "do not read or write". There is
 * no sensible device-wide fallback: writing one would put a profile's data back under a key the
 * next profile reads, which is the bug this exists to prevent.
 */
export function profileKey(base: string, userId: string | null | undefined): string | null {
  if (!userId) return null
  return `${base}.${userId}`
}

/**
 * Removes a pre-scoping, device-wide key.
 *
 * An upgrading device still has `hej.contacts.v1` sitting there with the last profile's directory in
 * it. Nothing reads it any more, which is not the same as it being gone — it is a readable cache of
 * other members' names and phone numbers, so it is deleted on first run rather than left to expire
 * with the origin.
 */
export function dropLegacyKey(storage: ScopedStorage | null, legacyKey: string) {
  if (!storage) return
  try {
    storage.removeItem(legacyKey)
  } catch {
    // Blocked or unavailable storage: nothing to clean up, and nothing worth failing over.
  }
}
