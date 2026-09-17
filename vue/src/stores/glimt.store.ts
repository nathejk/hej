import { defineStore } from 'pinia'

import { HttpError, fetchWrapper } from '@/helpers'
import { compressImage } from '@/helpers/glimtCompress'
import {
  countGlimtDrafts,
  enqueueGlimt,
  glimtDraftItems,
  listGlimtDrafts,
  markGlimtItemUploaded,
  outboxAvailable,
  recordGlimtDraftFailure,
  removeGlimtDraft,
  type GlimtDraft,
} from '@/helpers/glimtOutbox'
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

/** One hold in the browse index (PRD 019 §0a.1). */
export interface GlimtHoldSummary {
  number: string
  name: string
  group: string
  /** How many glimt from this hold the caller may see — not how many exist. */
  count: number
  latestAt: number
}

interface HoldsResponse {
  holds: Array<{
    number?: string
    name?: string
    group?: string
    count?: number
    latest_at?: string
  }> | null
  own_number?: string
}

/** What an upload returns, and what a create call references. */
export interface GlimtMediaRef {
  ref: string
  thumbRef: string
  kind: 'image' | 'video'
  width: number
  height: number
  bytes: number
  durationMs: number
}

interface StoredMediaResponse {
  ref: string
  thumb_ref?: string
  kind?: string
  width?: number
  height?: number
  bytes?: number
  duration_ms?: number
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
    /**
     * How many posts are waiting in the outbox (task 314).
     *
     * Held here so a view can say "1 glimt venter på nettet" without reading IndexedDB itself.
     * Refreshed by `refreshPending()` after anything that could change it.
     */
    pending: 0,
    /** True while a drain is running, so two cannot overlap. */
    draining: false,
    /**
     * The browse index: holds that have shared something visible to the caller.
     *
     * Not persisted, unlike the feed. See `holdGlimt` for why.
     */
    holds: [] as GlimtHoldSummary[],
    /**
     * The caller's own hold number, for the "din patrulje" shortcut. Empty for crew, who have a
     * section rather than a numbered hold — the UI then offers no shortcut rather than linking to a
     * collection that cannot exist.
     */
    ownHoldNumber: '',
    /**
     * One hold's collection, keyed by hold number, **oldest first** as the BFF returns it.
     *
     * # Held in memory only, deliberately
     *
     * The feed is persisted because it is the surface a member opens cold and offline. A hold
     * collection is not: it is reached by tapping an attribution, which means the feed already
     * loaded. Persisting these would write the same glimt again under one key per hold — the feed's
     * data duplicated N times — for a case the feed's own cache already covers.
     *
     * What actually makes revisiting a hold cheap offline is the **image** cache (task 315), which is
     * where the bytes are. Metadata for one hold is a few kilobytes and refetches in one request.
     */
    holdGlimt: {} as Record<string, Glimt[]>,
    /** True while a hold collection is loading, so the grid can say so. */
    loadingHold: false,
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
     * Load the browse index: which holds have shared something (PRD 019 §0a.1).
     *
     * Never throws. An empty index and a failed request look the same to a member who has just tapped
     * "se alle hold", so the error is recorded and the view says so rather than showing an
     * indistinguishable blank.
     */
    async fetchHolds(): Promise<boolean> {
      try {
        const data = await fetchWrapper.get<HoldsResponse>('/api/glimt/hold')
        this.holds = (data.holds ?? []).map((h) => ({
          number: h.number ?? '',
          name: h.name ?? '',
          group: h.group ?? '',
          count: h.count ?? 0,
          latestAt: h.latest_at ? Date.parse(h.latest_at) : 0,
        }))
        this.ownHoldNumber = data.own_number ?? ''
        this.error = ''
        return true
      } catch {
        this.error = 'Kunne ikke hente holdene.'
        return false
      }
    },

    /**
     * Load one hold's collection, oldest first.
     *
     * The order comes from the BFF (task 319) and is **not** re-sorted here: a race reads forward in
     * time, and there is exactly one place that decides so.
     */
    async fetchHold(number: string): Promise<boolean> {
      if (!number) return false
      this.loadingHold = true
      try {
        const data = await fetchWrapper.get<FeedResponse>(
          `/api/glimt/hold/${encodeURIComponent(number)}`,
        )
        this.holdGlimt = { ...this.holdGlimt, [number]: (data.glimt ?? []).map(toGlimt) }
        this.error = ''
        return true
      } catch (err) {
        if (err instanceof HttpError && err.status === 503) {
          this.error = 'Glimt er ikke tilgængelige lige nu.'
        } else {
          this.error = 'Kunne ikke hente holdets glimt.'
        }
        return false
      } finally {
        this.loadingHold = false
      }
    },

    /**
     * Queue a glimt for sending, and try to send it now.
     *
     * **This is the composer's entry point**, replacing a direct `uploadMedia` + `create` (task 314).
     * The difference is the promise PRD 019 §5 makes: the files are written to IndexedDB *before*
     * anything is attempted, so a post survives a failed upload, a locked phone and an app the OS
     * killed. What the member sees is the same either way — the drawer closes — which is the point: a
     * post is accepted, and delivery is our problem rather than theirs.
     *
     * Returns whether the glimt reached the server *now*. `false` is not a failure: it means queued,
     * and the feed shows it waiting.
     */
    async queue(input: {
      caption: string
      audience: Glimt['audience']
      files: Array<{ blob: Blob; name: string }>
    }): Promise<{ queued: boolean; sent: boolean }> {
      if (!outboxAvailable()) {
        // No IndexedDB — a node test run, or a browser that has blocked it. Fall back to sending
        // directly, because the alternative is refusing to post at all. Reported as not queued, so
        // a caller can say plainly that this one will not survive being closed.
        const sent = await this.sendNow(input)
        return { queued: false, sent }
      }

      const id = crypto.randomUUID()
      try {
        await enqueueGlimt(
          { id, caption: input.caption, audience: input.audience, createdAt: Date.now() },
          input.files,
        )
      } catch (err) {
        // Could not even queue it — almost always quota. Try to send directly rather than lose the
        // post, and say so.
        this.error = isQuotaExceeded(err)
          ? 'Der er ikke plads på telefonen til at gemme glimtet. Prøver at sende det nu.'
          : 'Kunne ikke gemme glimtet lokalt. Prøver at sende det nu.'
        const sent = await this.sendNow(input)
        return { queued: false, sent }
      }

      await this.refreshPending()
      const sent = await this.drain()
      return { queued: true, sent }
    },

    /**
     * Send everything in the outbox, oldest first.
     *
     * Called after queueing, and on foreground and `online` by the sync loop. **Never by Background
     * Sync**: it is unavailable on iOS and a backgrounded web app does not run there (PRD 002
     * measured 2% coverage), so anything implying otherwise would be a lie about where the member's
     * photographs are.
     *
     * Overlap-guarded, because foreground and `online` fire together often enough — unlock a phone in
     * a coverage hole and both arrive within a second.
     *
     * Oldest first, and it **stops at the first draft that fails**. Continuing would burn a data
     * budget re-failing on the same dead connection, and the queue is ordered because the member
     * posted in an order.
     */
    async drain(): Promise<boolean> {
      if (this.draining || !outboxAvailable()) return false
      this.draining = true
      let sentAny = false
      try {
        for (const draft of await listGlimtDrafts()) {
          const ok = await this.sendDraft(draft)
          if (!ok) break
          sentAny = true
        }
      } catch {
        // Reading the outbox failed. Nothing to do but leave it for the next trigger — and
        // deliberately not surfaced, because a member who is not posting does not need to hear
        // about it.
      } finally {
        this.draining = false
        await this.refreshPending()
      }
      return sentAny
    },

    /**
     * Send one queued draft: upload the items that have not landed, then create the glimt.
     *
     * Items already uploaded are skipped — their refs are on the row and their Blobs have been
     * dropped — so a draft that failed on item four resumes at item four rather than starting over.
     * That is the whole reason the outbox stores refs per item, and on rural mobile data it is the
     * difference between a post that eventually lands and one that never does.
     */
    async sendDraft(draft: GlimtDraft): Promise<boolean> {
      try {
        const items = await glimtDraftItems(draft.id)
        const refs: GlimtMediaRef[] = []

        for (const item of items) {
          if (item.uploaded) {
            refs.push(item.uploaded)
            continue
          }
          if (!item.blob) {
            // Neither bytes nor refs: nothing can be done with this item ever. Skipped rather
            // than failing the draft forever — the other photographs are still worth posting.
            continue
          }
          const stored = await this.uploadMedia(
            new File([item.blob], item.name || 'glimt.jpg', { type: item.blob.type }),
          )
          await markGlimtItemUploaded(draft.id, item.ordinal, stored)
          refs.push(stored)
        }

        if (refs.length === 0) {
          // A draft with nothing left to post. Removed rather than retried forever.
          await removeGlimtDraft(draft.id)
          return true
        }

        const created = await this.createFromRefs({
          caption: draft.caption,
          audience: draft.audience,
          media: refs,
        })
        if (!created.ok) {
          // Recorded on the draft, not raised as a banner: this is a background attempt.
          await recordGlimtDraftFailure(draft.id, created.reason)
          return false
        }

        await removeGlimtDraft(draft.id)
        return true
      } catch (err) {
        const message =
          err instanceof HttpError ? `HTTP ${err.status}` : 'netværksfejl'
        await recordGlimtDraftFailure(draft.id, message)
        // Not surfaced as `error`: a queued post that has not gone yet is a normal state on this
        // network, and an error banner on every foreground would train people to ignore it.
        return false
      }
    },

    /** Refresh the waiting count. */
    async refreshPending() {
      this.pending = await countGlimtDrafts()
    },

    /**
     * Discard a queued post.
     *
     * The member's decision, and the only way a draft leaves the outbox unsent. Nothing here gives up
     * on a post by itself — that is the promise.
     */
    async discardPending(draftId: string) {
      try {
        await removeGlimtDraft(draftId)
      } finally {
        await this.refreshPending()
      }
    },

    /**
     * Upload and create in one go, with no outbox involved.
     *
     * The fallback for a platform with no IndexedDB. Kept separate from `sendDraft` rather than
     * folded in, so the queued path has no branch that skips persistence.
     */
    async sendNow(input: {
      caption: string
      audience: Glimt['audience']
      files: Array<{ blob: Blob; name: string }>
    }): Promise<boolean> {
      try {
        const refs: GlimtMediaRef[] = []
        for (const file of input.files) {
          refs.push(
            await this.uploadMedia(
              new File([file.blob], file.name || 'glimt.jpg', { type: file.blob.type }),
            ),
          )
        }
        const created = await this.createFromRefs({
          caption: input.caption,
          audience: input.audience,
          media: refs,
        })
        if (!created.ok) {
          // A member is watching this one — it is the no-outbox path, reached straight from the
          // composer — so the reason becomes the banner.
          this.error = created.reason
          return false
        }
        this.error = ''
        return true
      } catch {
        this.error = 'Glimtet kunne ikke sendes. Prøv igen.'
        return false
      }
    },

    /**
     * Upload one media item and return what the create call will reference.
     *
     * One request per item rather than one big multipart post, matching the BFF (task 303): a glimt
     * carries up to ten items from a field on one bar of signal, so a failure should cost one item
     * rather than the whole post.
     *
     * Throws on failure, unlike everything else in this store. The composer needs to know *which*
     * item failed so it can mark that thumbnail and let the member retry or drop it — a swallowed
     * error would leave a post silently missing a photograph.
     */
    async uploadMedia(file: File): Promise<GlimtMediaRef> {
      const compressed = await compressImage(file)
      const form = new FormData()
      // The field is `media`, not `photo`: it carries video too (task 322), and the BFF names it
      // that way.
      form.append('media', compressed, file.name || 'glimt.jpg')
      const stored = await fetchWrapper.postForm<StoredMediaResponse>('/api/glimt/media', form)
      return {
        ref: stored.ref,
        thumbRef: stored.thumb_ref ?? '',
        kind: stored.kind === 'video' ? 'video' : 'image',
        width: stored.width ?? 0,
        height: stored.height ?? 0,
        bytes: stored.bytes ?? 0,
        durationMs: stored.duration_ms ?? 0,
      }
    },

    /**
     * Create a glimt from media already uploaded.
     *
     * Prepends the new glimt to the local copy on success rather than refetching. The member has
     * just watched their upload finish; a round trip before their own post appears would read as the
     * post having failed.
     *
     * Named `createFromRefs` rather than `create` because it is the *second half* of posting and takes
     * refs, not files. The composer calls `queue()`; this is what the drain eventually reaches.
     *
     * **Returns a reason and does not touch `this.error`.** `error` is UI state — a banner — and its
     * two callers want opposite things from a failure: a member watching the composer should be told,
     * while a background drain must stay quiet, because a queued post that has not gone yet is a
     * normal state on this network and a banner on every foreground would train people to ignore it.
     * Letting this function set the banner made the drain shout; a test caught it.
     */
    async createFromRefs(input: {
      caption: string
      audience: Glimt['audience']
      media: GlimtMediaRef[]
    }): Promise<{ ok: boolean; reason: string }> {
      try {
        const created = await fetchWrapper.post<GlimtResponse>('/api/glimt', {
          caption: input.caption,
          audience: input.audience,
          media: input.media.map((m) => ({
            ref: m.ref,
            thumb_ref: m.thumbRef,
            kind: m.kind,
            width: m.width,
            height: m.height,
            bytes: m.bytes,
            duration_ms: m.durationMs,
          })),
        })
        this.glimt = [toGlimt(created), ...this.glimt]
        await this.persist()
        return { ok: true, reason: '' }
      } catch (err) {
        if (err instanceof HttpError && err.status === 503) {
          // The stream is down. The BFF said so honestly rather than pretending (task 304), so the
          // client must too — and this is the message that tells a member to keep the photos and
          // try again rather than to take them a second time.
          return { ok: false, reason: 'Glimtet kunne ikke deles lige nu. Prøv igen om lidt.' }
        }
        return { ok: false, reason: 'Glimtet kunne ikke deles. Prøv igen.' }
      }
    },

    /**
     * Delete one of the caller's own glimt.
     *
     * Removes it locally on success rather than refetching: the feed is already on screen, and a
     * round trip would leave the card sitting there for a beat after the member asked for it to be
     * gone. The next sync check reconciles anyway.
     *
     * Only the author may do this and the BFF enforces it (task 306) — this does not re-check, so
     * that there is one authority rather than two that can disagree.
     */
    async remove(id: string): Promise<boolean> {
      try {
        await fetchWrapper.delete(`/api/glimt/items/${encodeURIComponent(id)}`)
        this.glimt = this.glimt.filter((g) => g.id !== id)
        await this.persist()
        this.error = ''
        return true
      } catch {
        // Kept on screen. Telling a member their glimt was deleted when it was not would be the
        // one lie that matters here.
        this.error = 'Kunne ikke slette glimtet. Prøv igen.'
        return false
      }
    },

    /**
     * Report a glimt.
     *
     * The reported glimt is **removed from this device's copy** on success, because the server has
     * hidden it from every audience the reporter belongs to (task 307) — leaving it on screen would
     * have the member watch the thing they objected to stay put, and invite them to report it again.
     *
     * A failure is surfaced rather than swallowed. A report that silently failed is the worst lie
     * this feature could tell: the member believes they have acted, and the photograph stays up.
     */
    async report(id: string, reason = ''): Promise<boolean> {
      try {
        await fetchWrapper.post(`/api/glimt/items/${encodeURIComponent(id)}/report`, { reason })
        this.glimt = this.glimt.filter((g) => g.id !== id)
        await this.persist()
        this.error = ''
        return true
      } catch {
        this.error = 'Kunne ikke anmelde glimtet. Prøv igen.'
        return false
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
