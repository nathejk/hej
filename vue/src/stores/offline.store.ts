import { defineStore } from 'pinia'

import {
  OFFLINE_DATASETS,
  OFFLINE_ORIGIN_BUDGET_BYTES,
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
}

/** What a dataset owner reports. Everything optional: report what you know. */
export type OfflineDatasetReport = Partial<OfflineDatasetStatus>

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
     * Whether the browser promised not to evict us. Null until asked (task 185).
     *
     * Three states, not two: "denied" and "never asked" call for different words in the
     * readiness view, and only one of them is worth telling the user about.
     */
    persisted: null as boolean | null,
    /** True when the app is working from cache rather than the network (task 188). */
    servingFromCache: false,
    /** Datasets dropped to reclaim space, most recent first (task 186). */
    evicted: [] as OfflineDatasetId[],
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

    /** True when at least one dataset has been reported on. Distinguishes cold from empty. */
    hydrated(state): boolean {
      return OFFLINE_DATASETS.some((d) => state.statuses[d.id].state !== 'unknown')
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
     * Record that a dataset's payload is gone although it had synced.
     *
     * Kept as its own action rather than a `report({ state: 'evicted' })` call because the
     * consequence is different: the last-synced time is *retained*. "You had this an hour ago
     * and the phone removed it" is a very different sentence from "you never had this", and it
     * is the one iOS makes us say (PRD 009 §5).
     */
    markEvicted(id: OfflineDatasetId) {
      this.statuses[id] = { ...this.statuses[id], state: 'evicted', complete: false, bytes: 0 }
      if (!this.evicted.includes(id)) this.evicted.unshift(id)
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
        if (storage.persisted) this.persisted = await storage.persisted()
      } catch {
        // A private-mode browser can throw here. Leaving the values null is honest: the
        // readiness view then says it does not know, rather than showing a confident zero.
      }
    },

    setServingFromCache(fromCache: boolean) {
      this.servingFromCache = fromCache
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
