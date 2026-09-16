import { defineStore } from 'pinia'

import { fetchWrapper } from '@/helpers'
import { profileKey, type ScopedStorage } from '@/helpers/profileStorage'
import { useSessionStore } from '@/stores/session.store'
import { versionedRefresh } from '@/stores/syncVersions'

// handouts.store holds the map sheets this patrol has been handed (PRD 016).
//
// # What this data is
//
// `GET /api/patrol/handouts` is patrol-scoped: the QR-bound sheets the patrol scanned in, plus sheets
// handed over at a post (synthesised, with no sticker number). It is the answer to the original problem
// this feature exists for — a patrol on the phone to base, trying to say which sheet they are holding.
//
// # Never names another team
//
// A sheet the patrol no longer holds arrives with `stillHeld === false` and **nothing else**. HQ's
// organizer view shows "Flyttet til {team}"; a participant may not see that, so the BFF never sends it and
// there is no field here to carry it. The absence is deliberate, not an oversight.
//
// # Never throws / replace, never merge / per profile
//
// The same three contracts the checkpoints store states, for the same reasons: the map must stay usable
// when a refresh fails (it keeps the cached copy and records why), a refetch replaces the whole list so a
// sheet the server stopped sending stops existing, and the cache is keyed per profile so a sibling
// switching in on a shared phone does not inherit the previous patrol's sheets.

export interface Handout {
  /** Sheet name, or "Ukendt kort" — the BFF decides the fallback, so a patrol and an organizer read one word. */
  name: string
  /** a4 | a3 | skitse | andet, or '' when the sheet is unknown. */
  format: string
  /**
   * The printed sticker number, or '' for a synthesised handout (a skitse has no code at all).
   *
   * Empty is the branch the drawer uses to lay a row out without a gap where the number would go.
   */
  qrId: string
  /** When the patrol was handed it. */
  handedOut: Date
  /** False once the sheet has been reassigned; the drawer shows "afleveret" and names nobody. */
  stillHeld: boolean
}

// The BFF speaks snake_case; mapped at the store boundary, as everywhere else here.
interface HandoutResponse {
  name: string
  format: string
  qr_id: string
  handed_out: string
  still_held: boolean
}

const STORAGE_BASE = 'hej.handouts.v1'

// The schema version travels in the payload as well as the key. Same reasoning as the checkpoints store:
// bumping the key orphans the old value, while the field lets a later build recognise and discard it.
const SCHEMA = 1

// Stored form: the handout with its date flattened to epoch ms, because a Date does not survive JSON.
interface StoredHandout {
  name: string
  format: string
  qrId: string
  handedOutMs: number
  stillHeld: boolean
}

interface StoredPayload {
  schema: number
  syncedAt: number
  handouts: StoredHandout[]
  /**
   * The sync version of the stored copy (PRD 017), or absent on a copy written before versions
   * existed.
   *
   * Persisted with the payload rather than kept in memory, and the reason is the cold start: without
   * it, every launch would refetch every dataset before the sync check could say "unchanged", which
   * is the cost this whole design exists to remove. Read as '' when missing, which reads as "nothing
   * held" and simply refetches once.
   */
  version?: string
}

// `localStorage` is absent in a node test run and *throws on access* — not on use, on access — in some
// Safari privacy modes, so even reading the global needs a guard. Returning null keeps every call site to
// one null check rather than a try/catch each.
function browserStorage(): ScopedStorage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}

function readStored(storage: ScopedStorage | null, key: string | null): StoredPayload | null {
  if (!storage || !key) return null
  try {
    const raw = storage.getItem(key)
    if (!raw) return null
    const parsed = JSON.parse(raw) as StoredPayload
    // What comes out of storage is input, not state: a half-written value, or one from a schema this
    // build does not know, is discarded rather than trusted.
    if (!parsed || parsed.schema !== SCHEMA || !Array.isArray(parsed.handouts)) return null
    return parsed
  } catch {
    return null
  }
}

function writeStored(storage: ScopedStorage | null, key: string | null, payload: StoredPayload) {
  if (!storage || !key) return
  try {
    storage.setItem(key, JSON.stringify(payload))
  } catch {
    // Safari throws on write when a quota is exceeded. The drawer works from memory regardless.
  }
}

export const useHandoutsStore = defineStore('handouts', {
  state: () => ({
    handouts: [] as Handout[],
    loading: false,
    loaded: false,
    error: '',
    /** When the held copy was fetched, epoch ms. Zero when nothing has ever synced. */
    syncedAt: 0,
    /** The version of the copy we hold, from `/api/sync`. Opaque: compared for equality only. */
    version: '',
    storage: browserStorage() as ScopedStorage | null,
  }),

  getters: {
    /**
     * The storage key for the signed-in profile, or null when nobody is.
     *
     * Null means "do not touch storage". No device-wide fallback on purpose: writing one would put this
     * profile's sheets under a key the next profile reads.
     */
    storageKey: (): string | null => profileKey(STORAGE_BASE, useSessionStore().user?.userId),

    hasAny: (state) => state.handouts.length > 0,
  },

  actions: {
    /**
     * Load the cached copy, if there is one. Called before `fetch` so the drawer can show sheets
     * immediately — offline, or on a slow link at 02:00.
     */
    hydrate() {
      const stored = readStored(this.storage, this.storageKey)
      if (!stored) return
      this.handouts = stored.handouts.map((h) => ({
        name: h.name,
        format: h.format,
        qrId: h.qrId,
        handedOut: new Date(h.handedOutMs),
        stillHeld: h.stillHeld,
      }))
      this.syncedAt = stored.syncedAt
      this.version = stored.version ?? ''
      this.loaded = true
    },

    /**
     * Fetch the patrol's handouts.
     *
     * Never throws. An empty list is a normal answer — a patrol before its first handout, and every
     * personnel user without a patrol — and is stored as such, so the drawer knows the difference between
     * "nothing handed out" and "never synced".
     *
     * Returns whether it succeeded, so a versioned caller knows whether it may record the version it
     * fetched against (`syncVersions.ts`).
     */
    async fetch(): Promise<boolean> {
      this.loading = true
      try {
        const data = await fetchWrapper.get<{ handouts: HandoutResponse[] | null }>(
          '/api/patrol/handouts',
        )
        // Replace, never merge: a sheet the server stopped sending has been reassigned away.
        this.handouts = (data.handouts ?? []).map(
          (h): Handout => ({
            name: h.name,
            format: h.format,
            qrId: h.qr_id,
            handedOut: new Date(h.handed_out),
            stillHeld: h.still_held,
          }),
        )
        this.syncedAt = Date.now()
        this.error = ''
        this.loaded = true
        writeStored(this.storage, this.storageKey, {
          schema: SCHEMA,
          syncedAt: this.syncedAt,
          version: this.version,
          handouts: this.handouts.map((h) => ({
            name: h.name,
            format: h.format,
            qrId: h.qrId,
            handedOutMs: h.handedOut.getTime(),
            stillHeld: h.stillHeld,
          })),
        })
        return true
      } catch {
        // The cached copy stays. Danish, and specific: "we could not refresh" is a different thing from
        // "you have no sheets".
        this.error = 'Kunne ikke opdatere kort.'
        return false
      } finally {
        this.loading = false
      }
    },

    /**
     * Refetch the sheets when the server's version differs from ours (PRD 017).
     *
     * The dataset this matters most for: a patrol handed Kort 3 at a post must see its sheet — and
     * through it, its checkpoints — without restarting the app.
     *
     * The version is stored *before* `fetch` writes the payload, so the two land in storage together;
     * a version written afterwards would need a second write and could be interrupted between them,
     * leaving a copy labelled with a version it does not have.
     */
    async refreshIfVersionDiffers(version: string): Promise<boolean> {
      // Hydrate first, or a cold start would compare against an empty version and refetch sheets we
      // already hold. Guarded, so hydrating cannot overwrite a fresher in-memory copy.
      if (!this.loaded) this.hydrate()

      const previous = this.version
      this.version = version
      const refreshed = await versionedRefresh(previous, version, () => this.fetch())
      if (!refreshed) {
        // Either nothing to do (same version) or the fetch failed. In the failure case the held copy
        // is still the old one, so the old version has to go back — otherwise we would hold stale
        // sheets labelled as current and never ask again.
        this.version = previous
      }
      return refreshed
    },
  },
})
