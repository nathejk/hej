import { defineStore } from 'pinia'

import { dropLegacyKey, profileKey } from '@/helpers/profileStorage'
import { useContactsStore, type ContactsStorage } from '@/stores/contacts.store'
import { useSessionStore } from '@/stores/session.store'

// Favourites in the contacts pane (PRD 007 §6/§7, task 166).
//
// # Device-local, by decision
//
// Maintainer direction, 2026-08-31: favourites live on the device. No endpoint, no server-side
// record of who is interested in whom — which is more sensitive than it first sounds, and is a
// thing we would then have to hold, justify and purge. The cost is that they do not survive a
// reinstall, and that is accepted.
//
// # Ids only
//
// Nothing but person ids is stored. Names and numbers are resolved against the synced directory
// at render time, so a favourite can never display a stale copy of somebody the user has since
// lost access to — a role change mid-event is exactly the case that would produce one.
//
// # Validated on read
//
// What comes out of storage is input, not state, for the same reason `config/roles.ts` validates
// a role read back from localStorage. A half-written array from a killed tab must not reach the
// render path.

// # Per profile, not per device
//
// Keyed by the signed-in profile (task 180). Two profiles on one handset — the case a shared number
// creates — must not inherit each other's favourites: switching to a sibling and finding *their*
// starred colleagues is both wrong and a small disclosure. Note `pruneAgainstDirectory` would
// eventually drop the ones the new profile cannot see, but only after a sync and only for people who
// are no longer visible, so it is not a substitute for keying.

const STORAGE_BASE = 'hej.contacts.favourites.v1'

// The device-wide key favourites used to live under; removed on first run.
const LEGACY_STORAGE_KEY = 'hej.contacts.favourites.v1'

function readIds(storage: ContactsStorage | null, key: string | null): string[] {
  if (!storage || !key) return []
  try {
    const raw = storage.getItem(key)
    if (!raw) return []

    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    // Filtered rather than rejected wholesale: one bad element should cost that element, not
    // somebody's whole favourites list.
    return parsed.filter((v): v is string => typeof v === 'string' && v.length > 0)
  } catch {
    return []
  }
}

function writeIds(storage: ContactsStorage | null, key: string | null, ids: string[]) {
  if (!storage || !key) return
  try {
    storage.setItem(key, JSON.stringify(ids))
  } catch {
    // Blocked or full storage costs persistence, not the session.
  }
}

export const useFavouritesStore = defineStore('contactsFavourites', {
  state: () => ({
    /** Person ids, in the order they were added. */
    ids: [] as string[],
    hydrated: false,
    /**
     * Storage seam, mirroring the contacts store: `vitest.config.ts` runs tests in `node`
     * because modules here take their browser surface as an argument rather than reading
     * globals.
     */
    storage: null as ContactsStorage | null,
  }),
  getters: {
    /** Storage key for the signed-in profile; null means "do not touch storage". */
    storageKey: (): string | null => profileKey(STORAGE_BASE, useSessionStore().user?.userId),

    has: (state) => (id: string) => state.ids.includes(id),
    count: (state) => state.ids.length,
  },
  actions: {
    /**
     * Loads the stored ids. The storage seam is passed in by the caller (the pane), so this
     * store does not need to know how the platform exposes storage.
     */
    hydrate(storage: ContactsStorage | null) {
      if (this.hydrated) return
      this.hydrated = true
      this.storage = storage
      dropLegacyKey(storage, LEGACY_STORAGE_KEY)
      this.ids = readIds(storage, this.storageKey)
    },

    toggle(id: string) {
      if (!id) return
      this.ids = this.ids.includes(id) ? this.ids.filter((v) => v !== id) : [...this.ids, id]
      writeIds(this.storage, this.storageKey, this.ids)
    },

    /**
     * Drops favourites the user may no longer see.
     *
     * Called after every sync. A favourite must not outlive the user's permission to see that
     * person: a crew member reassigned to a different function, or a klan member whose role
     * changes, would otherwise keep a row for somebody now out of scope.
     *
     * **A withdrawn member is not out of scope.** They stay in the manifest with a status
     * marking and no phone number (task 160), so they remain a favourite — losing them from
     * favourites the moment they go home is exactly when a samarit might be looking for them.
     * That distinction is why this prunes against "is in the manifest" rather than against
     * `stillInRace`.
     *
     * Does nothing while the directory is empty, which matters: an offline start that has not
     * synced yet must not be read as "you may see nobody" and wipe the list.
     */
    pruneAgainstDirectory() {
      const contacts = useContactsStore()
      if (contacts.entries.length === 0) return

      const visible = contacts.byId
      const kept = this.ids.filter((id) => visible.has(id))
      if (kept.length === this.ids.length) return

      this.ids = kept
      writeIds(this.storage, this.storageKey, kept)
    },
  },
})
