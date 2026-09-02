// The offline storage budget: which datasets this device keeps, how much room each gets, and
// which one gets dropped when the origin runs out (PRD 009 §6, task 183).
//
// # Why this is data and not code
//
// Four caches already exist and none of them can see the others: map tiles in the Cache API,
// the contacts directory in localStorage, the position track in IndexedDB, the app shell in the
// Workbox precache. The browser evicts **per origin**, not per cache, so "what do we sacrifice"
// is a question none of them can answer alone. This module is the answer, in one place, so the
// eviction policy (task 186) and the readiness view (task 187) read a declaration instead of
// each re-deciding.
//
// PRD 009 deliberately did **not** build a dataset registry or a generic sync engine (§4,
// §11.2): the consumers had already shipped, differently and reasonably. So nothing here
// registers, wraps or owns storage. It is policy those caches observe.
//
// # Why it lives in config/ rather than helpers/offline/
//
// PRD 009 §8 said `helpers/offline/`. The numbers ended up here instead, alongside
// `config/cache.ts`, for the reason that file gives: it must stay importable with **no
// browser-only imports**, because `vite.config.ts` reads it at build time. The tile budget is
// already split between that file (entry cap) and this one (bytes), and putting them in
// different halves of the tree would guarantee they drift. Logic — eviction, adapters — still
// goes in `helpers/offline/`.

/**
 * Which storage a dataset lives in.
 *
 * Three kinds, deliberately (PRD 009 §8). Tiles belong in the Cache API because a service
 * worker serves them from there; the track belongs in IndexedDB because it is the only data
 * here that cannot be re-fetched, so it must not sit in a 5 MB synchronous store; the
 * directory is small enough that localStorage is honest. Unifying them would be churn with no
 * user-visible benefit.
 */
export type OfflineStorageKind = 'cache-api' | 'indexeddb' | 'local-storage'

export type OfflineDatasetId = 'track' | 'shell' | 'directory' | 'portraits' | 'tiles'

export interface OfflineDataset {
  id: OfflineDatasetId
  /** Danish, for the readiness view. */
  label: string
  /**
   * What this data is *for*, in one plain-Danish sentence, addressed to the person whose phone it is.
   *
   * Lives here rather than in a template because two pages ask about the same data and must not
   * describe it differently: the profile page's readiness section (task 187) and the privacy page's
   * "what is on your phone" (task 196). The privacy page's voice is the stricter of the two — readable
   * by a 12-year-old and their parent, no jargon — so that is the register these are written in.
   */
  purpose: string
  kind: OfflineStorageKind
  /**
   * Bytes planned for this dataset.
   *
   * A *plan*, not a limit the browser enforces. Its job is to make the sum checkable against
   * the ceiling and to give the readiness view a denominator.
   */
  budgetBytes: number
  /**
   * True when eviction means **data loss** rather than a re-download.
   *
   * The distinction the whole order exists for. Exactly one dataset has it today; the flag
   * exists so the next one is handled by construction rather than by someone remembering.
   */
  unrecoverable: boolean
  /**
   * True when the data is personal and must not outlive the event.
   *
   * Drives the server-issued expiry and the post-event purge (task 193). Note it is
   * independent of `unrecoverable`: the track is unrecoverable but not sensitive in this
   * sense — it is the user's own recording awaiting upload, not someone else's details being
   * retained.
   */
  sensitive: boolean
}

/**
 * Every cached dataset, **most protected first**.
 *
 * Array order *is* the priority order — there is no `rank` field, because two sources for one
 * fact drift. Eviction walks this list from the bottom (see `evictionOrder`).
 *
 * The order was confirmed by the maintainer on 2026-09-01 (PRD 009 §6, §11.1). The one real
 * trade in it: **tiles are what gets sacrificed.** They are ~99% of the bytes, and everything
 * above them together is under 15 MB, so protecting the rest costs almost nothing. The
 * counter-argument is real and was weighed — evicting a tile is irreversible in the field,
 * since a tile discarded in a dead spot cannot be re-fetched (PRD 002 §11.2) — but a lost tile
 * costs a participant a map they can manage without, while a lost track is a recording that
 * never existed.
 */
export const OFFLINE_DATASETS: readonly OfflineDataset[] = [
  {
    id: 'track',
    label: 'Din rute',
    purpose: 'Vejen du er gået, så I kan se den bagefter. Den gemmes her, indtil den er sendt.',
    kind: 'indexeddb',
    // Budgeted against the *hard ceiling* (TRACK_MAX_POINTS = 20,000 points ≈ 2.7 MB), not the
    // expected 12-hour race (~1,440 points ≈ 195 KB). Everything else here is budgeted for
    // what it normally holds, because exceeding it just means a re-download; this one is never
    // evicted, so its budget has to cover the worst case it is allowed to reach.
    budgetBytes: 3 * 1024 * 1024,
    unrecoverable: true,
    sensitive: false,
  },
  {
    id: 'shell',
    label: 'Appen selv',
    purpose: 'Selve appen, så den kan åbne, når der ikke er signal.',
    kind: 'cache-api',
    // **Measured** in task 192: Workbox precaches 47 entries / 738 kB, including the icon and splash
    // set from task 091. 2 MB rather than 0.8 leaves room for the app to grow without a re-plan,
    // and it is small enough against the tile budget that precision here buys nothing.
    budgetBytes: 2 * 1024 * 1024,
    unrecoverable: false,
    sensitive: false,
  },
  {
    id: 'directory',
    label: 'Kontaktliste',
    purpose:
      'Navne, grupper og telefonnumre på dem, du er i løb sammen med, så du kan finde dem uden signal.',
    kind: 'local-storage',
    // The text index only; portraits are the entry below. Ranked above them on purpose: names,
    // groups and status must keep working when the images are gone, which is the point of
    // keeping index and binaries apart (PRD 009 §6).
    budgetBytes: 1024 * 1024,
    unrecoverable: false,
    sensitive: true,
  },
  {
    id: 'portraits',
    label: 'Portrætter',
    purpose: 'Små billeder af de andres ansigter, så I kan genkende hinanden i mørket.',
    kind: 'cache-api',
    // ~4.5 kB per face at thumb256 (task 104): ~0.7 MB for the largest single role, ~3.7 MB
    // event-wide (PRD 007 §8). 5 MB matches the cache's own ceiling —
    // `PORTRAIT_CACHE_MAX_ENTRIES` × 4.5 kB — so the two cannot disagree about what "full" means.
    budgetBytes: 5 * 1024 * 1024,
    unrecoverable: false,
    sensitive: true,
  },
  {
    id: 'tiles',
    label: 'Kortbilleder',
    purpose: 'Kortet omkring løbsområdet, så du kan se det uden signal.',
    kind: 'cache-api',
    // The 2026 race area at z12–16 measures 324 MB across both base layers. 500 MB is planned
    // rather than 324 so a larger year (600 km² ≈ 446 MB) does not need a re-plan; the area is
    // "roughly the same size every year" but not identical.
    budgetBytes: 500 * 1024 * 1024,
    unrecoverable: false,
    sensitive: false,
  },
] as const

/**
 * The whole-origin planning ceiling, in bytes.
 *
 * ~1 GB, which is the **iOS 16.4–16.7** per-origin quota (raised in 200 MB prompts under the
 * pre-Safari-17 policy). iOS 17+ and Android Chrome both give 60% of total disk, so the old
 * floor is the binding one — and `.rules` puts the baseline at iOS 16.4. Planning to the floor
 * makes modern devices a non-issue rather than a separate case.
 *
 * It is an **upper bound with no guarantee**: quota is best-effort and evictable even after
 * `navigator.storage.persist()` succeeds, so `QuotaExceededError` is expected rather than
 * unreachable (task 186).
 */
export const OFFLINE_ORIGIN_BUDGET_BYTES = 1024 * 1024 * 1024

/**
 * Order in which map tiles are discarded: **highest zoom first**.
 *
 * Not arbitrary. z16 alone is 60% of the tile bytes, and it is the least information per byte
 * to lose in the sense that matters here: z12–14 is the orientation view a lost participant
 * actually needs, at 56 MB for the whole race area, while z15–16 is 268 MB of detail. Dropping
 * from the top degrades the map gradually instead of punching holes in it.
 *
 * z17 is absent because it is never cached: DTK25 is a 1:25.000 product with a native 1.25 m/px
 * resolution — that is z16 — so z17 is the same cartography upsampled, costing bytes and
 * carrying no new information.
 */
export const TILE_EVICTION_ZOOM_ORDER: readonly number[] = [16, 15, 14, 13, 12]

/** Lookup by id. Throws rather than returning undefined: every id is a literal in this file. */
export function offlineDataset(id: OfflineDatasetId): OfflineDataset {
  const found = OFFLINE_DATASETS.find((d) => d.id === id)
  if (!found) throw new Error(`unknown offline dataset: ${id}`)
  return found
}

/**
 * Datasets in the order they may be evicted: least protected first, and **never** anything
 * unrecoverable.
 *
 * Unrecoverable datasets are filtered out here rather than merely ranked last, so a caller
 * cannot reach them by walking one step too far.
 */
export function evictionOrder(): OfflineDataset[] {
  return [...OFFLINE_DATASETS].reverse().filter((d) => !d.unrecoverable)
}

/** Sum of the per-dataset plans. Must fit inside `OFFLINE_ORIGIN_BUDGET_BYTES`. */
export function totalPlannedBytes(): number {
  return OFFLINE_DATASETS.reduce((sum, d) => sum + d.budgetBytes, 0)
}
