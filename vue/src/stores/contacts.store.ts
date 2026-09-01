import { defineStore } from 'pinia'

import { HttpError, fetchWrapper } from '@/helpers'
import { dropLegacyKey, profileKey } from '@/helpers/profileStorage'
import { useSessionStore } from '@/stores/session.store'

// The contacts directory, cached on the device (PRD 007).
//
// # Deliberately thin, and deliberately not a sync engine
//
// PRD 007 §8 says the sync mechanism belongs to PRD 009, which is still a draft. Rather than
// invent a generic dataset layer that 009 would then have to displace — the duplication 009
// exists to prevent — this store does exactly one job for exactly one dataset. When 009 lands,
// it should be able to replace the fetch and persist calls in here without the pane noticing
// (task 177; the registration itself is task 161).
//
// So: no plugin points, no dataset registry, no eviction policy, no storage budget.
//
// # Index only, no image cache
//
// Portraits are `<img src="/api/contacts/people/{id}/photo">`, served with
// `Cache-Control: private, max-age=3600` and content-hash ETags, so the browser and service
// worker cache them. Hand-rolling a blob cache here would duplicate that badly.
//
// # Version comes from the body, not a header
//
// Both endpoints return `version` in their JSON, so this store never needs response headers —
// which `fetchWrapper` does not expose. The ETags the BFF also sets are for the browser's own
// conditional requests, not for us.

/** One person as listed in one group. A person may appear twice; see `population`. */
export interface ContactEntry {
  id: string
  name: string
  /** Which list this row belongs to: bandit, gøgler or crew. Never spejder. */
  population: string
  /** Grouping path, outermost first. One level today; a klan, "Gøglere" or "Crew". */
  groups: ContactGroup[]
  /** The person's own number, never a guardian's. Empty for a withdrawn member. */
  phone?: string
  /** Section label, for crew only. */
  crewFunction?: string
  /** False once the member has left the race: show a marking, offer no call. */
  stillInRace: boolean
  /** Thumbnail content hash, or absent when there is no portrait. */
  portraitVersion?: string
}

export interface ContactGroup {
  id: string
  label: string
  isOwn: boolean
}

interface ManifestResponse {
  version: string
  entries: ContactEntry[] | null
}

interface VersionResponse {
  version: string
}

/** One rendered group with its members, ready for the accordion. */
export interface ContactGroupView {
  population: string
  group: ContactGroup
  entries: ContactEntry[]
}

const STORAGE_BASE = 'hej.contacts.v1'

// The device-wide key this data used to live under. Removed on first run — see profileStorage.
const LEGACY_STORAGE_KEY = 'hej.contacts.v1'

// The schema version is part of the stored payload as well as the key, which looks redundant
// and is not: bumping the key orphans the old value until the browser evicts it, while the
// field lets a future version recognise and discard what it finds. Cheap insurance against a
// shape change reaching a device that skipped a release.
const SCHEMA = 1

interface StoredPayload {
  schema: number
  version: string
  syncedAt: number
  entries: ContactEntry[]
}

/**
 * The subset of `Storage` this store uses.
 *
 * A seam rather than a direct `localStorage` reference, for the reason `vitest.config.ts`
 * gives for running tests in `node`: "the modules under test take their browser environment
 * as an argument rather than reading globals". Tests assign a fake; production gets the real
 * thing, or null where it is unavailable.
 */
export interface ContactsStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

// Resolved once, defensively. `localStorage` is absent in a node test run and *throws* on
// access in some Safari privacy modes — not on use, on access — so even reading the global
// needs a guard. Returning null rather than a throwing stub keeps every call site to one
// null check instead of a try/catch each.
function browserStorage(): ContactsStorage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}

// Every access is still wrapped even with the seam: Safari can throw on write when a quota
// is exceeded, and this runs on a route the user may land on cold and offline. An exception
// here would white-screen the pane, which is task 090's lesson.
function readStored(storage: ContactsStorage | null, key: string | null): StoredPayload | null {
  if (!storage || !key) return null
  try {
    const raw = storage.getItem(key)
    if (!raw) return null

    const parsed = JSON.parse(raw) as unknown
    if (!isStoredPayload(parsed)) return null
    if (parsed.schema !== SCHEMA) return null
    return parsed
  } catch {
    // Malformed JSON, a quota error, a blocked API: all mean "no local copy", never "crash".
    return null
  }
}

function writeStored(storage: ContactsStorage | null, key: string | null, payload: StoredPayload) {
  if (!storage || !key) return
  try {
    storage.setItem(key, JSON.stringify(payload))
  } catch {
    // Out of quota or blocked. The in-memory copy still works for this session, and the
    // pane reports when it last synced, so failing quietly here is honest rather than lossy.
  }
}

function clearStored(storage: ContactsStorage | null, key: string | null) {
  if (!storage || !key) return
  try {
    storage.removeItem(key)
  } catch {
    // Nothing to do; callers clear the in-memory copy regardless.
  }
}

// Validated rather than trusted, for the same reason `config/roles.ts` validates a role read
// back from localStorage: what comes out of storage is input, not state. A half-written value
// from a killed tab must not reach the render path as `undefined.groups`.
function isStoredPayload(v: unknown): v is StoredPayload {
  if (typeof v !== 'object' || v === null) return false
  const p = v as Partial<StoredPayload>
  return (
    typeof p.schema === 'number' &&
    typeof p.version === 'string' &&
    typeof p.syncedAt === 'number' &&
    Array.isArray(p.entries) &&
    p.entries.every(isContactEntry)
  )
}

function isContactEntry(v: unknown): v is ContactEntry {
  if (typeof v !== 'object' || v === null) return false
  const e = v as Partial<ContactEntry>
  return (
    typeof e.id === 'string' &&
    typeof e.name === 'string' &&
    typeof e.population === 'string' &&
    typeof e.stillInRace === 'boolean' &&
    Array.isArray(e.groups)
  )
}

export const useContactsStore = defineStore('contacts', {
  state: () => ({
    entries: [] as ContactEntry[],
    /** The version of the copy we hold. Empty when nothing has ever synced. */
    version: '',
    /** When the copy was fetched, as epoch ms. Null when never. */
    syncedAt: null as number | null,
    loading: false,
    /** True once hydration has run, so the pane can tell "empty" from "not read yet". */
    hydrated: false,
    /**
     * True when the BFF says this role has no contacts pane (403).
     *
     * Distinct from an error: there is nothing to retry and nothing to apologise for. A
     * spejder should not see a failure state for a feature that is not theirs.
     */
    forbidden: false,
    /** Set when a refresh failed. The stored copy is still shown. */
    error: '',
    /**
     * Where the local copy lives. Null when the platform has no storage — the pane then works
     * for the session and simply cannot survive a reload.
     */
    storage: browserStorage() as ContactsStorage | null,
  }),
  getters: {
    /**
     * The storage key for the signed-in profile, or null when nobody is signed in.
     *
     * Null means "do not touch storage": there is no device-wide fallback on purpose, because
     * writing one would put this profile's directory back under a key the next profile reads
     * (task 180).
     */
    storageKey: (): string | null => profileKey(STORAGE_BASE, useSessionStore().user?.userId),

    hasCopy: (state) => state.entries.length > 0,

    /**
     * Entries grouped for rendering, own group first.
     *
     * Grouping keys on the *last* element of the path, which is the innermost group — one
     * level today, but a lok/klan pair later resolves to the klan, which is what a row
     * belongs to either way.
     */
    groupViews: (state): ContactGroupView[] => {
      const views = new Map<string, ContactGroupView>()

      for (const entry of state.entries) {
        const group = entry.groups[entry.groups.length - 1]
        if (!group) continue

        const key = `${entry.population}/${group.id}`
        const existing = views.get(key)
        if (existing) existing.entries.push(entry)
        else views.set(key, { population: entry.population, group, entries: [entry] })
      }

      // Own group first, then by label. The caller's own klan is the one they open the pane
      // for, so it goes to the top and (per PRD 007) opens by default.
      return [...views.values()].sort((a, b) => {
        if (a.group.isOwn !== b.group.isOwn) return a.group.isOwn ? -1 : 1
        return a.group.label.localeCompare(b.group.label, 'da')
      })
    },

    /** Distinct people, for lookups that should not see a person twice. */
    byId: (state): Map<string, ContactEntry> => {
      const out = new Map<string, ContactEntry>()
      for (const e of state.entries) if (!out.has(e.id)) out.set(e.id, e)
      return out
    },
  },
  actions: {
    /** Loads the stored copy. Safe to call repeatedly; only the first call reads storage. */
    hydrate() {
      if (this.hydrated) return
      this.hydrated = true

      // One-time cleanup of the pre-scoping key, which still holds the last profile's directory
      // on an upgrading device.
      dropLegacyKey(this.storage, LEGACY_STORAGE_KEY)

      const stored = readStored(this.storage, this.storageKey)
      if (!stored) return

      this.entries = stored.entries
      this.version = stored.version
      this.syncedAt = stored.syncedAt
    },

    /**
     * Fetches the manifest and **replaces** the stored copy.
     *
     * Replace, never merge. A withdrawn member's phone number has to disappear from a device
     * that already synced it (PRD 007 §11.6, task 160); merging would keep the old number
     * forever and make the purge decorative. The whole payload is small enough — well under a
     * megabyte for the largest role — that there is nothing to gain from a partial update.
     *
     * Never throws. The pane's premise is working at 03:00 with no signal, so a failed
     * refresh keeps what we have and records why.
     */
    async fetch() {
      this.hydrate()
      this.loading = true
      try {
        const data = await fetchWrapper.get<ManifestResponse>('/api/contacts/manifest')

        this.entries = data.entries ?? []
        this.version = data.version
        this.syncedAt = Date.now()
        this.forbidden = false
        this.error = ''

        writeStored(this.storage, this.storageKey, {
          schema: SCHEMA,
          version: this.version,
          syncedAt: this.syncedAt,
          entries: this.entries,
        })
      } catch (err) {
        if (err instanceof HttpError && err.status === 403) {
          // Not an error: this role has no contacts pane. Clear the copy, because a role can
          // change mid-event and anything we hold is now out of scope.
          this.forbidden = true
          this.entries = []
          this.version = ''
          this.syncedAt = null
          this.error = ''
          clearStored(this.storage, this.storageKey)
          return
        }
        this.error = 'Kunne ikke opdatere kontakter.'
      } finally {
        this.loading = false
      }
    },

    /**
     * Asks whether our copy is current, and refetches only if not.
     *
     * This is the cheap call the freshness loop makes (task 162): a few hundred bytes against
     * a version endpoint the BFF answers from a short-lived cache. Returns true when a
     * refetch happened, so a caller can log or report it.
     */
    async refreshIfStale(): Promise<boolean> {
      this.hydrate()

      // Nothing held yet: skip the version check and go straight for the payload.
      if (!this.version) {
        await this.fetch()
        return true
      }

      try {
        const data = await fetchWrapper.get<VersionResponse>('/api/contacts/version')
        if (data.version === this.version) {
          // Current. Not an error, and not a sync either — deliberately does not touch
          // syncedAt, because "we checked" and "we refetched" are different facts and the UI
          // shows the second one.
          this.error = ''
          return false
        }
      } catch (err) {
        if (err instanceof HttpError && err.status === 403) {
          await this.fetch()
          return false
        }
        this.error = 'Kunne ikke opdatere kontakter.'
        return false
      }

      await this.fetch()
      return true
    },
  },
})
