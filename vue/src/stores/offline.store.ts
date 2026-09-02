import { defineStore } from 'pinia'

import {
  reclaim,
  type EvictionResult,
  type EvictorMap,
} from '@/helpers/offline/eviction'
import {
  requestPersistence,
  type PersistenceOutcome,
  type PersistenceStorageManager,
} from '@/helpers/offline/persistence'
import {
  OFFLINE_DATASETS,
  OFFLINE_ORIGIN_BUDGET_BYTES,
  offlineDataset,
  type OfflineDatasetId,
  totalPlannedBytes,
} from '@/config/offline'

// The aggregate picture of what this device has cached (PRD 009 §8, task 184).
//
// # It holds no data of its own
//
// Every cached dataset keeps its own storage and its own fetching: tiles in a Workbox route,
// the directory in `contacts.store`, the track in `trackDb`. This store holds only their
// *status*, so the readiness view (task 187) and the shell indicator (task 188) have one place
// to read instead of interrogating four subsystems.
//
// That asymmetry is the whole shape of PRD 009 after its rescope (§4, §11.2): shared policy and
// shared reporting, not shared storage. Anything here that starts fetching or evicting is a sign
// the registry has been reinvented.
//
// # Why "not read yet" is a state
//
// Four states, and the boring-looking one matters most. `unknown` means nobody has reported yet;
// `empty` means the dataset has genuinely never synced. Conflating them is exactly how an app
// looks empty while it is merely still loading — task 090's white screen, and the failure PRD
// 009 §9 measures against. `contacts.store` already keeps a `hydrated` flag for the same reason.
//
// `evicted` is the fifth and the one nobody expects: metadata says this synced, the payload is
// gone. On iOS that is normal rather than exceptional, and the user is owed an explanation
// instead of a blank page.

// # Connectivity is not kept here
//
// There is deliberately no `online` or `servingFromCache` flag in this store. `app.store.online`
// already owns that (task 090) and owns it better: it is seeded from `navigator.onLine` and then
// *corrected by what actually happens*, because `onLine` only means "there is a network interface
// with a route" — true on a captive portal, true with one bar and no throughput, true on the
// event's own patchy coverage. A second copy here would drift from it, and the two would disagree
// in exactly the situation both exist for. See task 188.

export type OfflineDatasetState =
  /** Nobody has reported on this dataset yet. Not the same as empty. */
  | 'unknown'
  /** Reported, and there is nothing stored. Never synced, or deliberately cleared. */
  | 'empty'
  /** Stored and current. */
  | 'synced'
  /** Stored, usable, and old enough that the UI should say when it was fetched. */
  | 'stale'
  /** We synced this and the payload is no longer there. The OS took it. */
  | 'evicted'
  /** Being fetched right now. */
  | 'syncing'

export interface OfflineDatasetStatus {
  state: OfflineDatasetState
  /** Epoch ms of the last successful sync, or null if never. */
  syncedAt: number | null
  /** Rows, tiles, faces — whatever the dataset counts. Null when it does not count. */
  itemCount: number | null
  /** Measured bytes, when the dataset can measure itself. Null when it cannot. */
  bytes: number | null
  /**
   * True when the dataset holds everything it is supposed to.
   *
   * Separate from `synced` on purpose: a partially fetched tile set is stored, current and
   * *incomplete*, and reporting it as synced is the dishonesty PRD 009 exists to prevent.
   */
  complete: boolean
  /** Server-issued expiry, epoch ms (task 193). Null when the data does not expire. */
  expiresAt: number | null
  /**
   * How far a running fetch has got, or null when nothing is running.
   *
   * Only meaningful for a dataset whose work is countable — the tile download (task 087) knows it has
   * 5,291 tiles to get. A dataset that is one request keeps this null rather than reporting 0/1, so the
   * view can tell "working, indeterminate" from "working, 12% done".
   */
  progress: { done: number; total: number } | null
  /**
   * Why the last attempt did not finish, when it did not.
   *
   * Two causes worth telling a user apart, because the responses differ: `'quota'` means the phone is
   * full and they must free space or accept less map; `'offline'` means try again with signal. Silence
   * would leave a download that stopped at 60% looking like one that finished.
   */
  problem: 'quota' | 'offline' | null
}

/** What a dataset owner reports. Everything optional: report what you know. */
export type OfflineDatasetReport = Partial<OfflineDatasetStatus>

/**
 * What a dataset owner can be asked to do on the user's behalf.
 *
 * A seam, not a registry. The store never learns *how* a dataset is fetched or stored — it only
 * holds a function to call when the user taps "hent nu", so the readiness view can offer a control
 * without importing five feature stores and knowing which of them has a refresh method.
 *
 * A dataset with no handler simply gets no button, which is the right default: the app shell has
 * nothing a user could usefully re-fetch, and the position track has nothing to fetch at all.
 */
export interface OfflineDatasetHandlers {
  /** Fetch or refresh. Should report progress and completion through `report`. */
  sync?: () => Promise<void>
  /** Delete this dataset's local copy. Never offered for unrecoverable data — see `clear`. */
  clear?: () => Promise<void> | void
  /**
   * Stop a running `sync`, keeping whatever it already stored.
   *
   * Only for work long enough that a user would want out of it — a 324 MB tile download can run for
   * minutes on rural mobile data, and a progress bar with no way to stop it is a trap.
   */
  cancel?: () => void
}

/**
 * The subset of `navigator.storage` this store uses.
 *
 * Taken as an argument rather than read as a global, for the reason `vitest.config.ts` gives:
 * the tests run in `node`, where there is no `navigator`, and the interesting cases here —
 * persistence denied, quota absent — cannot be reproduced on the machine running them anyway.
 */
export interface OfflineStorageManager {
  estimate?: () => Promise<StorageEstimate>
  persisted?: () => Promise<boolean>
}

function unknownStatus(): OfflineDatasetStatus {
  return {
    state: 'unknown',
    syncedAt: null,
    itemCount: null,
    bytes: null,
    complete: false,
    expiresAt: null,
    progress: null,
    problem: null,
  }
}

function initialStatuses(): Record<OfflineDatasetId, OfflineDatasetStatus> {
  const out = {} as Record<OfflineDatasetId, OfflineDatasetStatus>
  for (const dataset of OFFLINE_DATASETS) out[dataset.id] = unknownStatus()
  return out
}

export const useOfflineStore = defineStore('offline', {
  state: () => ({
    statuses: initialStatuses(),
    /**
     * Bytes the origin is using, and its quota, as the browser reports them.
     *
     * The only *truth* in this store — everything else is what features believe about
     * themselves. Worth preferring when the two disagree.
     */
    usageBytes: null as number | null,
    quotaBytes: null as number | null,
    /**
     * What the browser said about evicting us, or 'unknown' until asked (task 185).
     *
     * Four values, not a boolean: "denied" and "never asked" and "this browser does not do
     * persistence" call for different words in the readiness view, and only one of them is
     * worth telling the user about.
     */
    persistence: 'unknown' as 'unknown' | PersistenceOutcome,
    /** Datasets dropped to reclaim space, most recent first (task 186). */
    evicted: [] as OfflineDatasetId[],
    /**
     * Per-dataset sync/clear callbacks, registered by whoever owns the storage.
     *
     * Deliberately outside `statuses`: these are functions, not state, and keeping them apart
     * stops a devtools snapshot of this store from being full of closures.
     */
    handlers: {} as Partial<Record<OfflineDatasetId, OfflineDatasetHandlers>>,
  }),

  getters: {
    /** The sum of what datasets report, for the readiness view's total. */
    reportedBytes(state): number {
      return OFFLINE_DATASETS.reduce((sum, d) => sum + (state.statuses[d.id].bytes ?? 0), 0)
    },

    /** What the budget planned for, so a total can be shown against something. */
    plannedBytes(): number {
      return totalPlannedBytes()
    },

    /**
     * True only when every dataset is stored and complete.
     *
     * Deliberately strict, and deliberately not "no dataset is missing": `unknown` counts as
     * not ready. A readiness answer that defaults to yes before anything has reported is worse
     * than no answer, because it is the one a user acts on before walking into a forest.
     */
    ready(state): boolean {
      return OFFLINE_DATASETS.every((d) => {
        const status = state.statuses[d.id]
        return status.complete && (status.state === 'synced' || status.state === 'stale')
      })
    },

    /** Datasets a user would consider absent, for the readiness view to name. */
    missing(state): OfflineDatasetId[] {
      return OFFLINE_DATASETS.filter((d) => {
        const status = state.statuses[d.id]
        return status.state === 'empty' || status.state === 'evicted' || !status.complete
      }).map((d) => d.id)
    },

    /** True while any dataset is fetching, so one progress indicator can cover them all. */
    syncing(state): boolean {
      return OFFLINE_DATASETS.some((d) => state.statuses[d.id].state === 'syncing')
    },

    /** Percentage complete for whichever dataset is reporting countable progress, else null. */
    syncPercent(state): number | null {
      for (const dataset of OFFLINE_DATASETS) {
        const p = state.statuses[dataset.id].progress
        if (p && p.total > 0) return Math.min(100, Math.round((p.done / p.total) * 100))
      }
      return null
    },

    /** Datasets that can be stopped mid-sync. */
    cancellable(state): OfflineDatasetId[] {
      return OFFLINE_DATASETS.filter(
        (d) => state.handlers[d.id]?.cancel && state.statuses[d.id].state === 'syncing',
      ).map((d) => d.id)
    },

    /**
     * True when the phone may remove this data — the only persistence state worth a sentence.
     *
     * 'unsupported' is excluded deliberately: nothing the user can act on, and a warning about
     * a decision no browser made is noise that teaches people to ignore the surface.
     */
    evictable(state): boolean {
      return state.persistence === 'denied'
    },

    /** True when at least one dataset has been reported on. Distinguishes cold from empty. */
    hydrated(state): boolean {
      return OFFLINE_DATASETS.some((d) => state.statuses[d.id].state !== 'unknown')
    },

    /**
     * Datasets the user can ask to fetch, in the order a "prepare everything" run would do them.
     *
     * Declared order, which is cheap-first by construction: tiles rank last for eviction and so
     * come last here too, which is also what we want for a download — the small text datasets
     * finish in seconds and the 324 MB one is what a user on cellular may want to interrupt.
     */
    syncable(state): OfflineDatasetId[] {
      return OFFLINE_DATASETS.filter((d) => state.handlers[d.id]?.sync).map((d) => d.id)
    },

    /**
     * Roughly what a "prepare everything" run would download, in bytes.
     *
     * From the *planned* budgets, not from anything the server has told us, because the estimate
     * has to be on screen **before** the first request — on iOS the app cannot tell WiFi from
     * cellular (`navigator.connection` is unavailable in Safari), so this number is the entire
     * consent mechanism for a 324 MB download. An estimate the user sees beats an exact figure
     * they only get afterwards.
     *
     * Complete datasets are excluded; partially present ones are counted in full, which overstates
     * rather than understates. That direction is deliberate: a download that turns out smaller
     * than warned is a pleasant surprise, the reverse is a betrayal on a metered connection.
     */
    pendingBytes(state): number {
      return OFFLINE_DATASETS.filter(
        (d) => state.handlers[d.id]?.sync && !state.statuses[d.id].complete,
      ).reduce((sum, d) => sum + d.budgetBytes, 0)
    },
  },

  actions: {
    /**
     * The reporting seam. A dataset owner calls this; nothing here reaches into its storage.
     *
     * Merges rather than replaces, so a caller that knows only its byte count does not have to
     * invent a `syncedAt` — inventing one would put a wrong timestamp in front of the user,
     * which is worse than showing none.
     */
    report(id: OfflineDatasetId, report: OfflineDatasetReport) {
      this.statuses[id] = { ...this.statuses[id], ...report }
    },

    /**
     * Record that a dataset lost cached data to make room, or to the OS.
     *
     * Kept as its own action rather than a `report({ state: 'evicted' })` call because the
     * consequence is different: the last-synced time is *retained*. "You had this an hour ago
     * and the phone removed it" is a very different sentence from "you never had this", and it
     * is the one iOS makes us say (PRD 009 §5).
     *
     * `emptied` distinguishes the two cases the user experiences differently: the whole dataset
     * is gone, or some of it was trimmed. Trimming tiles is the normal outcome of a quota
     * eviction and leaves a perfectly usable map with a smaller area — calling that "evicted"
     * would overstate it, while calling it "synced" would hide that coverage shrank.
     */
    markEvicted(id: OfflineDatasetId, emptied = true) {
      this.statuses[id] = {
        ...this.statuses[id],
        state: emptied ? 'evicted' : this.statuses[id].state,
        complete: false,
        ...(emptied ? { bytes: 0 } : {}),
      }
      if (!this.evicted.includes(id)) this.evicted.unshift(id)
    },

    /**
     * Free space, in the declared priority order, and record what it cost.
     *
     * Called by a dataset owner that has just caught a `QuotaExceededError` — not on a timer and
     * not from a size watcher. Eviction runs when something actually needs room, because guessing
     * at "nearly full" from `navigator.storage.estimate()` means acting on a number iOS rounds
     * and pads.
     *
     * The evictors are handed in rather than built here: this store deliberately owns no storage
     * (task 184), and the adapters belong to the features that do.
     */
    async reclaimSpace(evictors: EvictorMap, targetBytes?: number): Promise<EvictionResult> {
      const result = await reclaim(evictors, targetBytes)
      for (const id of result.evicted) this.markEvicted(id, false)
      return result
    },

    /** Reset a dataset to "nothing stored", after a purge or a manual clear. */
    markCleared(id: OfflineDatasetId) {
      this.statuses[id] = {
        ...unknownStatus(),
        state: 'empty',
        // Retained: it still answers "when did this device last have it", which matters after
        // a post-event purge as much as after an eviction.
        syncedAt: this.statuses[id].syncedAt,
      }
    },

    /**
     * Ask for persistent storage, once per session, and remember the answer.
     *
     * Called at app level rather than from a feature: the request is per-origin, so one answer
     * covers every cache. PRD 009 asks for it "at install/onboarding", and the app's first mount
     * *is* onboarding for a new install — doing it on every mount instead of only in the welcome
     * flow also repairs devices that were onboarded before this shipped, which a
     * once-per-install hook would have left permanently evictable.
     *
     * Cheap to call repeatedly: `requestPersistence` short-circuits when already granted.
     */
    async ensurePersistence(storage: PersistenceStorageManager | undefined) {
      if (this.persistence === 'granted') return
      this.persistence = await requestPersistence(storage)
    },

    /**
     * Read the origin's real usage. Guarded, because `navigator.storage` is absent in some
     * engines and this must not be the thing that breaks a page.
     */
    async refreshStorage(storage: OfflineStorageManager | undefined) {
      if (!storage) return
      try {
        if (storage.estimate) {
          const estimate = await storage.estimate()
          this.usageBytes = estimate.usage ?? null
          this.quotaBytes = estimate.quota ?? null
        }
        // Only ever *upgrades* the answer. A `persisted()` of false does not distinguish "asked
        // and refused" from "not asked yet", so letting it write 'denied' here would invent a
        // refusal nobody made and put a warning in front of the user for it.
        if (storage.persisted && this.persistence === 'unknown' && (await storage.persisted())) {
          this.persistence = 'granted'
        }
      } catch {
        // A private-mode browser can throw here. Leaving the values null is honest: the
        // readiness view then says it does not know, rather than showing a confident zero.
      }
    },

    /** Register a dataset's sync/clear callbacks. Called by the feature that owns the storage. */
    registerHandlers(id: OfflineDatasetId, handlers: OfflineDatasetHandlers) {
      this.handlers[id] = { ...this.handlers[id], ...handlers }
    },

    /**
     * Fetch one dataset on the user's behalf.
     *
     * Sets `syncing` here rather than trusting the handler to, so the button cannot be left
     * looking idle by a feature that forgot — and restores the previous state on failure instead
     * of inventing 'empty', because a failed refresh does not remove what is already stored.
     */
    async sync(id: OfflineDatasetId) {
      const handler = this.handlers[id]?.sync
      if (!handler) return

      const previous = this.statuses[id].state
      this.statuses[id] = { ...this.statuses[id], state: 'syncing' }
      try {
        await handler()
      } catch {
        // The handler owns the error message; this only has to avoid lying about the state.
        if (this.statuses[id].state === 'syncing') {
          this.statuses[id] = { ...this.statuses[id], state: previous }
        }
      } finally {
        // Progress belongs to a *running* job. Leaving the last numbers behind would show a finished
        // download as though it were still at 4,912 of 5,291.
        this.statuses[id] = { ...this.statuses[id], progress: null }
      }
    },

    /** Stop a running sync, if this dataset offers a way to. */
    cancel(id: OfflineDatasetId) {
      this.handlers[id]?.cancel?.()
    },

    /**
     * Delete one dataset's local copy.
     *
     * **Refuses anything unrecoverable.** Not a confirmation dialog — a refusal. The only dataset
     * this applies to is the position track, whose local copy may be the sole record of where a
     * team was; a "free up space" button next to it is a foot-gun no amount of copy fixes, and the
     * track's own status page already has a considered path for it.
     */
    async clear(id: OfflineDatasetId) {
      if (offlineDataset(id).unrecoverable) return
      const handler = this.handlers[id]?.clear
      if (!handler) return
      await handler()
      this.markCleared(id)
    },

    /**
     * Fetch everything that can be fetched, cheapest first.
     *
     * Sequential on purpose. Parallel downloads over rural mobile data compete for the same thin
     * pipe and make the progress indicator meaningless, and the one dataset that takes minutes
     * (tiles) is the one a user is most likely to want to interrupt — which they cannot do if it
     * started at the same time as everything else.
     */
    async prepareAll() {
      for (const id of this.syncable) await this.sync(id)
    },

    /**
     * How much of the planning ceiling is left, in bytes, or null when unknowable.
     *
     * Uses the *smaller* of the browser's quota and our own ceiling. Our ceiling is the iOS
     * 16.4 floor and is deliberately conservative, but a device with a nearly full disk can
     * report less than that, and believing our own number there would plan a download the
     * phone cannot hold.
     */
    headroomBytes(): number | null {
      if (this.usageBytes === null) return null
      const ceiling =
        this.quotaBytes === null
          ? OFFLINE_ORIGIN_BUDGET_BYTES
          : Math.min(this.quotaBytes, OFFLINE_ORIGIN_BUDGET_BYTES)
      return Math.max(0, ceiling - this.usageBytes)
    },
  },
})
