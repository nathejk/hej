import { defineStore } from 'pinia'

import { fetchWrapper } from '@/helpers'
import { profileKey, type ScopedStorage } from '@/helpers/profileStorage'
import { useSessionStore } from '@/stores/session.store'

// checkpoints.store holds the checkpoints this patrol has earned sight of (PRD 016).
//
// # What this data is, and why the client does no filtering
//
// The BFF decides. `GET /api/checkpoints` is patrol-scoped: it returns the posts drawn on the sheets
// this patrol has been handed, plus those in checkgroups it has already reached, and nothing else. The
// positions of every other post never leave the server. So there is deliberately **no visibility flag
// on this data and nothing here to filter** — if a checkpoint is in this store, the patrol is allowed
// to see it. A client-side filter would imply the payload contained something it must not.
//
// # Never throws
//
// The map must stay usable when this fails, so a failed fetch keeps whatever is held and records why —
// the same contract `scans.store` states. On a race night the map is the thing people open when
// something has gone wrong; it is the last surface that may white-screen.
//
// # Replace, never merge
//
// A refetch replaces the whole list (PRD 009 §6). A merge would leave a withdrawn post on the map for
// the rest of the event, because nothing would ever tell the client it had gone — the server simply
// stops sending it.
//
// # Per profile, like everything else cached
//
// Several profiles share one phone. Keyed by user id (`profileKey`), so a sibling switching in does not
// inherit the previous patrol's map.

export interface Checkpoint {
  id: string
  name: string
  /** The postlinje group. Route order is decided by the BFF, which sends the list already sorted. */
  checkgroup: string
  sortOrder: number
  lat: number
  lng: number
  /**
   * The open window. Zero means "not set", which is a normal state and renders as no time — never as
   * 1970. A real window on this event is never near the epoch.
   */
  openFrom: number
  openUntil: number
  /** Minutes, for a relative window whose anchor is this patrol's scan at another group. */
  openDurationMinutes: number
}

// The BFF speaks snake_case; mapped at the store boundary, as everywhere else here.
interface CheckpointResponse {
  id: string
  name: string
  checkgroup: string
  sort_order: number
  lat: number
  lng: number
  open_from: number
  open_until: number
  open_duration_minutes: number
}

const STORAGE_BASE = 'hej.checkpoints.v1'

// The schema version travels in the payload as well as the key. Not redundant: bumping the key orphans
// the old value until the browser evicts it, while the field lets a later version recognise and discard
// what it finds. Same reasoning as the contacts store.
const SCHEMA = 1

interface StoredPayload {
  schema: number
  syncedAt: number
  checkpoints: Checkpoint[]
  nextCheckgroup: string
}

// `localStorage` is absent in a node test run and *throws on access* — not on use, on access — in some
// Safari privacy modes, so even reading the global needs a guard. Returning null keeps every call site
// to one null check rather than a try/catch each.
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
    if (!parsed || parsed.schema !== SCHEMA || !Array.isArray(parsed.checkpoints)) return null
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
    // Safari throws on write when a quota is exceeded. The map works from memory regardless, and
    // white-screening the one page people open when lost would be the worse outcome.
  }
}

export const useCheckpointsStore = defineStore('checkpoints', {
  state: () => ({
    checkpoints: [] as Checkpoint[],
    /**
     * The line the patrol is heading for, or '' when there is none.
     *
     * Decided by the BFF, not here. It needs route order across checkgroups *and* whether the patrol has
     * started — departing the start is recorded at check-in rather than as a scan at a post, and "has started"
     * has exactly one definition in the backend which a client-side copy would fork (task 275).
     */
    nextCheckgroup: '',
    loading: false,
    loaded: false,
    error: '',
    /** When the held copy was fetched, epoch ms. Zero when nothing has ever synced. */
    syncedAt: 0,
    storage: browserStorage() as ScopedStorage | null,
  }),

  getters: {
    /**
     * The storage key for the signed-in profile, or null when nobody is.
     *
     * Null means "do not touch storage". There is no device-wide fallback on purpose: writing one
     * would put this profile's map under a key the next profile reads.
     */
    storageKey: (): string | null => profileKey(STORAGE_BASE, useSessionStore().user?.userId),

    hasAny: (state) => state.checkpoints.length > 0,

    /**
     * Checkpoints by id, for joining a scan to the post it happened at.
     */
    byId: (state): Map<string, Checkpoint> => new Map(state.checkpoints.map((c) => [c.id, c])),

    /**
     * The posts to arrow: every revealed post in the next line.
     *
     * All of them, because a line holds several posts and the patrol chooses which to walk to when they get
     * there — the app must not nominate one for them (task 275).
     */
    nextLine: (state): Checkpoint[] =>
      state.nextCheckgroup === ''
        ? []
        : state.checkpoints.filter((c) => c.checkgroup === state.nextCheckgroup),
  },

  actions: {
    /**
     * Load the cached copy, if there is one.
     *
     * Called before `fetch` so the map can draw immediately — offline, or on a slow link at 02:00,
     * this is the difference between a map with posts on it and an empty one.
     */
    hydrate() {
      const stored = readStored(this.storage, this.storageKey)
      if (!stored) return
      this.checkpoints = stored.checkpoints
      this.nextCheckgroup = stored.nextCheckgroup ?? ''
      this.syncedAt = stored.syncedAt
      this.loaded = true
    },

    /**
     * Fetch the patrol's revealed checkpoints.
     *
     * Never throws. An empty list is a normal answer — a patrol before its first handout, and every
     * personnel user without a patrol — and is stored as such, so the map knows the difference between
     * "nothing revealed" and "never synced".
     */
    async fetch() {
      this.loading = true
      try {
        const data = await fetchWrapper.get<{
          checkpoints: CheckpointResponse[] | null
          next_checkgroup?: string
        }>('/api/checkpoints')
        // Replace, never merge: a post the server stopped sending must stop existing here.
        this.checkpoints = (data.checkpoints ?? []).map(
          (c): Checkpoint => ({
            id: c.id,
            name: c.name,
            checkgroup: c.checkgroup,
            sortOrder: c.sort_order,
            lat: c.lat,
            lng: c.lng,
            openFrom: c.open_from,
            openUntil: c.open_until,
            openDurationMinutes: c.open_duration_minutes,
          }),
        )
        this.nextCheckgroup = data.next_checkgroup ?? ''
        this.syncedAt = Date.now()
        this.error = ''
        this.loaded = true
        writeStored(this.storage, this.storageKey, {
          schema: SCHEMA,
          syncedAt: this.syncedAt,
          checkpoints: this.checkpoints,
          nextCheckgroup: this.nextCheckgroup,
        })
      } catch {
        // The cached copy stays. Danish, and specific: "we could not refresh" is a different thing
        // from "you have no posts", and the map is still useful in the first case.
        this.error = 'Kunne ikke opdatere poster.'
      } finally {
        this.loading = false
      }
    },
  },
})
