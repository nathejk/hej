import { defineStore } from 'pinia'

import { HttpError, fetchWrapper } from '@/helpers'
import { isQuotaExceeded } from '@/helpers/offline/eviction'
import { browserEvictors } from '@/helpers/offline/evictors'
import { profileKey } from '@/helpers/profileStorage'
import { useOfflineStore } from '@/stores/offline.store'
import { useSessionStore } from '@/stores/session.store'
import { isForbidden } from '@/stores/syncVersions'

// Glimt: the moments shared with this member (PRD 019).
//
// # Follows contacts.store.ts, and only differs where the data does
//
// The pattern is not re-argued here: profile-scoped schema-versioned key, runtime guards on read,
// server-issued deadline, a storage seam so tests run in node, replace-never-merge, and freshness by
// a `glimt` key on `/api/sync` rather than a poller of its own. Read `contacts.store.ts` for the
// reasoning behind each. What follows is what Glimt does differently.
//
// # Metadata here, media in the Cache API
//
// This store holds captions, attributions and ordinals — small text. The photographs are
// `<img src="/api/glimt/items/{id}/media/{ordinal}?variant=thumb">`, served `private, immutable`
// with content-hash ETags, so the browser and service worker cache them (task 315 gives them their
// own Workbox route and budget). Hand-rolling a blob cache here would duplicate that badly, and
// worse than for contacts: the volume is thousands of thumbnails rather than a few hundred faces.
//
// **The two halves expire together.** `expiresAt` is why: when a deadline passes, this store drops
// its copy *and* raises `expired`, so whoever owns the media cache can drop the images in the same
// beat. A feed of captions with no pictures and a pile of pictures with no feed are both worse than
// neither — and the second one is a pile of photographs of children that nothing points at.
//
// # No blob refs, by design
//
// The payload carries no content hashes (task 302). Media is addressed by `{id, ordinal}`, because a
// hash in a cached payload would be a forwardable capability that outlives every check.

/** How a glimt is attributed: the hold, never the person (PRD 019 §0b). */
export interface GlimtHold {
  /** Empty for crew, who have a section rather than a numbered hold. */
  number: string
  name: string
  /** spejder | bandit | crew — so a reader can tell Patrulje 42 from Klan 42. */
  group: string
}

/** One media item, addressed by ordinal. */
export interface GlimtMedia {
  ordinal: number
  kind: 'image' | 'video'
  width: number
  height: number
  /** 0 for a still. */
  durationMs: number
  /**
   * Whether a thumbnail exists. False is a real state, not a fault — an upload whose thumbnail
   * failed still has its photo (task 303) — and the grid must then ask for the full item rather
   * than rendering a gap.
   */
  hasThumb: boolean
}

export interface Glimt {
  id: string
  hold: GlimtHold
  /** True when the caller is the author. The payload never carries who that is. */
  own: boolean
  audience: 'group' | 'nathejk' | 'public'
  caption: string
  createdAt: number
  media: GlimtMedia[]
  /**
   * Taken down by a report or a moderator. Only ever true on a glimt the caller authored or
   * moderates — anyone else simply does not receive it — so it is safe to render, and an author is
   * entitled to know their post was hidden rather than silently discovering nobody can see it.
   */
  hidden: boolean
}

// The BFF speaks snake_case; mapped at this boundary as the skill requires.
interface GlimtResponse {
  id: string
  hold?: { number?: string; name?: string; group?: string }
  own?: boolean
  audience?: string
  caption?: string
  created_at?: string
  media?: Array<{
    ordinal?: number
    kind?: string
    width?: number
    height?: number
    duration_ms?: number
    has_thumb?: boolean
  }>
  hidden?: boolean
}

interface FeedResponse {
  glimt: GlimtResponse[] | null
  /** Server-issued deadline, epoch ms. Absent when retention is configured off. */
  expires_at?: number
}

const STORAGE_BASE = 'hej.glimt.v1'

// Schema version, in the payload as well as the key — the redundancy contacts.store.ts explains:
// bumping the key orphans the old value until the browser evicts it, while the field lets a future
// version recognise and discard what it finds.
const SCHEMA = 1

interface StoredPayload {
  schema: number
  version: string
  syncedAt: number
  /** Server-issued deadline, epoch ms. Zero when the server issued none. */
  expiresAt: number
  glimt: Glimt[]
}

/**
 * The subset of `Storage` this store uses.
 *
 * A seam rather than a direct `localStorage` reference, because the unit suite runs in `node` with
 * no DOM: the modules under test take their browser environment as an argument rather than reading
 * globals.
 */
export interface GlimtStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

// Resolved defensively. `localStorage` is absent in a node test run and *throws* on access in some
// Safari privacy modes — on access, not on use — so even reading the global needs a guard.
function browserStorage(): GlimtStorage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}

function readStored(storage: GlimtStorage | null, key: string | null): StoredPayload | null {
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
    // Task 090's lesson — an exception on a route a user may land on cold and offline
    // white-screens the app.
    return null
  }
}

function writeStored(storage: GlimtStorage | null, key: string | null, payload: StoredPayload) {
  if (!storage || !key) return true
  try {
    storage.setItem(key, JSON.stringify(payload))
    return true
  } catch (err) {
    // Returns whether it landed, so the caller can make room and retry once.
    return !isQuotaExceeded(err)
  }
}

function clearStored(storage: GlimtStorage | null, key: string | null) {
  if (!storage || !key) return
  try {
    storage.removeItem(key)
  } catch {
    // Nothing to do; callers clear the in-memory copy regardless.
  }
}

// Validated rather than trusted: what comes out of storage is input, not state. A half-written value
// from a killed tab must not reach the render path as `undefined.media`.
function isStoredPayload(v: unknown): v is StoredPayload {
  if (typeof v !== 'object' || v === null) return false
  const p = v as Partial<StoredPayload>
  return (
    typeof p.schema === 'number' &&
    typeof p.version === 'string' &&
    typeof p.syncedAt === 'number' &&
    typeof p.expiresAt === 'number' &&
    Array.isArray(p.glimt) &&
    p.glimt.every(isGlimt)
  )
}

function isGlimt(v: unknown): v is Glimt {
  if (typeof v !== 'object' || v === null) return false
  const g = v as Partial<Glimt>
  return (
    typeof g.id === 'string' &&
    typeof g.audience === 'string' &&
    typeof g.caption === 'string' &&
    typeof g.createdAt === 'number' &&
    typeof g.own === 'boolean' &&
    typeof g.hold === 'object' &&
    g.hold !== null &&
    Array.isArray(g.media)
  )
}

const AUDIENCES = ['group', 'nathejk', 'public'] as const

/**
 * Map one BFF glimt into the client shape.
 *
 * Every field is defaulted, because this runs on a payload that may have been written by a newer or
 * older BFF than the bundle expects. An unrecognised audience falls back to `group` — the narrowest
 * — so a value this client does not understand cannot be *rendered* as more widely shared than it is.
 * The server is the authority on who actually sees it; this only decides what chip to draw.
 */
function toGlimt(r: GlimtResponse): Glimt {
  const audience = AUDIENCES.find((a) => a === r.audience) ?? 'group'
  return {
    id: r.id,
    hold: {
      number: r.hold?.number ?? '',
      name: r.hold?.name ?? '',
      group: r.hold?.group ?? '',
    },
    own: r.own === true,
    audience,
    caption: r.caption ?? '',
    createdAt: r.created_at ? Date.parse(r.created_at) : 0,
    media: (r.media ?? []).map((m) => ({
      ordinal: typeof m.ordinal === 'number' ? m.ordinal : 0,
      kind: m.kind === 'video' ? 'video' : 'image',
      width: m.width ?? 0,
      height: m.height ?? 0,
      durationMs: m.duration_ms ?? 0,
      hasThumb: m.has_thumb === true,
    })),
    hidden: r.hidden === true,
  }
}

/**
 * The URL for one media item.
 *
 * Built here rather than in a component so there is one definition of the path, and so the
 * thumbnail decision lives next to the `hasThumb` flag that informs it. `variant=thumb` is what the
 * hold grid asks for — the post-race browse pulls these by the thousand over a congested network
 * (PRD 019 §0a.3), and asking for full-size media per tile is the difference between a usable grid
 * and an unusable one.
 */
export function glimtMediaUrl(glimtId: string, ordinal: number, variant: 'full' | 'thumb'): string {
  const base = `/api/glimt/items/${encodeURIComponent(glimtId)}/media/${ordinal}`
  return variant === 'thumb' ? `${base}?variant=thumb` : base
}

export const useGlimtStore = defineStore('glimt', {
  state: () => ({
    glimt: [] as Glimt[],
    /** The version of the copy we hold. Empty when nothing has ever synced. */
    version: '',
    /** When the copy was fetched, as epoch ms. Null when never. */
    syncedAt: null as number | null,
    /** Server-issued deadline for this copy, epoch ms. Zero when the server issued none. */
    expiresAt: 0,
    loading: false,
    /** True once hydration has run, so a view can tell "empty" from "not read yet". */
    hydrated: false,
    /**
     * True when hydration found a copy past its deadline and threw it away.
     *
     * Recorded rather than silently handled, because something else must act on it: the thumbnails
     * live in a Cache API bucket this store knows nothing about, and they have to go at the same
     * time. A flag is how the two halves of one purge stay one purge.
     */
    expired: false,
    /** True when the BFF says this caller gets no feed (403). Not an error. */
    forbidden: false,
    /** Set when a refresh failed. The stored copy is still shown. */
    error: '',
    storage: browserStorage() as GlimtStorage | null,
  }),
  getters: {
    /**
     * The storage key for the signed-in profile, or null when nobody is signed in.
     *
     * Null means "do not touch storage". There is no device-wide fallback on purpose: writing one
     * would put this profile's feed under a key the next profile reads, and a bandit must not find
     * the crew's group-scoped photographs cached on a shared phone (PRD 012, task 180).
     */
    storageKey: (): string | null => profileKey(STORAGE_BASE, useSessionStore().user?.userId),

    hasCopy: (state) => state.glimt.length > 0,

    /** Newest first, as the BFF returns them. Sorted defensively so a stored copy cannot drift. */
    newestFirst: (state): Glimt[] => [...state.glimt].sort((a, b) => b.createdAt - a.createdAt),

    /** The caller's own glimt, for a "dine glimt" affordance. */
    own: (state): Glimt[] => state.glimt.filter((g) => g.own),
  },
  actions: {
    /** Write the copy to storage, making room first if it does not fit. */
    async persist() {
      const payload: StoredPayload = {
        schema: SCHEMA,
        version: this.version,
        syncedAt: this.syncedAt ?? Date.now(),
        expiresAt: this.expiresAt,
        glimt: this.glimt,
      }

      if (writeStored(this.storage, this.storageKey, payload)) return

      // A feed that will not fit is not a lost cause: PRD 009's priority order says map tiles may
      // be sacrificed for data that cannot be re-derived. One retry — a second failure means
      // something other than tiles is filling the origin, and looping would only delay saying so.
      const result = await useOfflineStore().reclaimSpace(
        browserEvictors(typeof caches === 'undefined' ? undefined : caches),
      )
      if (result.freedBytes === 0) return

      writeStored(this.storage, this.storageKey, payload)
    },

    /** Drop this device's copy, keeping the session's in-memory one intact. */
    clearLocalCopy() {
      clearStored(this.storage, this.storageKey)
      this.syncedAt = null
      this.version = ''
      this.expiresAt = 0
    },

    /** Loads the stored copy. Safe to call repeatedly; only the first call reads storage. */
    hydrate() {
      if (this.hydrated) return
      this.hydrated = true

      const stored = readStored(this.storage, this.storageKey)
      if (!stored) return

      // Expired: throw it away rather than show it. This is the check that actually enforces
      // retention on a **dormant device** — a phone that never reopened the app after the event,
      // where no purge job, service worker or push will ever run again. It is the only lever we
      // hold over a cache full of photographs of children.
      //
      // A server-issued deadline compared against `Date.now()` is as good as it gets: a device with
      // its clock set back keeps the copy a while longer but cannot extend the deadline itself, and
      // one set forward discards early. Bounded either way, which a client-computed TTL cannot claim.
      if (stored.expiresAt > 0 && Date.now() > stored.expiresAt) {
        clearStored(this.storage, this.storageKey)
        this.expired = true
        return
      }

      this.glimt = stored.glimt
      this.version = stored.version
      this.syncedAt = stored.syncedAt
      this.expiresAt = stored.expiresAt
    },

    /**
     * Fetch the feed and **replace** the stored copy.
     *
     * Replace, never merge. A deleted glimt, a hidden one, or one the retention sweep purged must
     * stop existing on a device that already synced it (PRD 009 §6) — merging would keep it forever
     * and make every takedown decorative. That matters more here than for contacts: the thing being
     * removed is a photograph somebody objected to.
     *
     * Never throws. The premise is a phone at 03:00 with no signal, so a failed refresh keeps what
     * we have and records why.
     */
    async fetch(): Promise<boolean> {
      this.hydrate()
      this.loading = true
      try {
        const data = await fetchWrapper.get<FeedResponse>('/api/glimt/feed')

        this.glimt = (data.glimt ?? []).map(toGlimt)
        this.expiresAt = data.expires_at ?? 0
        this.syncedAt = Date.now()
        this.forbidden = false
        this.error = ''

        await this.persist()
        return true
      } catch (err) {
        if (isForbidden(err)) {
          // Not an error: this caller gets no feed. Clear the copy, because a role can change
          // mid-event and anything held is now out of scope.
          this.forbidden = true
          this.glimt = []
          this.version = ''
          this.syncedAt = null
          this.expiresAt = 0
          this.error = ''
          clearStored(this.storage, this.storageKey)
          return false
        }
        if (err instanceof HttpError && err.status === 503) {
          // The BFF distinguishes "unavailable" from "empty" deliberately (task 304), and the
          // client must keep that distinction: showing an empty feed here would let a device cache
          // "nobody shared anything tonight" because the database was down.
          this.error = 'Glimt er ikke tilgængelige lige nu.'
          return false
        }
        this.error = 'Kunne ikke opdatere glimt.'
        return false
      } finally {
        this.loading = false
      }
    },

    /**
     * Refetch when the server's version differs from ours (PRD 017).
     *
     * The version is **handed** to this store by the multiplexed `/api/sync` check rather than asked
     * for per dataset, so the common case — nothing changed — costs no request at all.
     *
     * Unlike contacts, the feed payload carries no `version` of its own, so the version held is the
     * one this method was given, and it is recorded **only after a successful fetch**. That is the
     * subtle rule `syncVersions.ts` exists to state: storing it after a failed refetch would leave
     * the device holding old data labelled current, and it would never ask again.
     */
    async refreshIfVersionDiffers(version: string): Promise<boolean> {
      this.hydrate()

      // A caller with no feed must not be nudged into asking again on every foreground.
      if (this.forbidden) return false

      if (this.version && this.version === version) {
        // Current. Deliberately does not touch `syncedAt`: "we checked" and "we refetched" are
        // different facts, and the UI shows the second.
        this.error = ''
        return false
      }

      const refreshed = await this.fetch()
      if (refreshed) this.version = version
      return refreshed
    },
  },
})
