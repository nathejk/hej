// Offline cache configuration.
//
// Kept in its own module with **no browser-only imports**, because `vite.config.ts`
// imports it too: the Workbox runtime-caching rule and any in-app code that inspects or
// purges the cache must agree on the name, and two copies of a string like this drift
// silently — the symptom is a cache that fills up and is never read.

/**
 * Cache holding Dataforsyningen map tiles.
 *
 * Its own cache rather than the precache, for three reasons: it is measured separately in
 * the readiness view (PRD 009), it is purged separately after the event, and it is the only
 * cache large enough that a mistake in it is expensive.
 */
export const TILE_CACHE_NAME = 'nathejk-map-tiles-v1'

/**
 * Maximum number of tiles retained.
 *
 * Sized from the measured race area (PRD 002 §11.2): the whole area at z12–16 is 5,291
 * tiles for one base layer, and a participant may browse more than one layer. 12,000 gives
 * room for that plus incidental browsing outside the area.
 *
 * The arithmetic matters because Workbox's expiration is by **entry count, not bytes**, and
 * tile sizes vary by an order of magnitude across zooms (133 kB at z12, 41 kB at z16 for
 * topo; ~11 kB for aerial JPEG). At a realistic mix this cap lands around 400–500 MB, which
 * is the budget PRD 009 plans for. Raising it without redoing that sum is how the cache
 * quietly grows past the iOS 16 ceiling.
 *
 * Eviction is least-recently-used. PRD 009 notes eviction is irreversible in the field — a
 * tile discarded in a dead spot cannot be re-fetched — which is the reason this cap is
 * generous rather than tight: browsing outside the race area must not evict the area itself.
 */
export const TILE_CACHE_MAX_ENTRIES = 12_000

/**
 * How long a cached tile is considered usable, in seconds.
 *
 * A year. The topographic maps are revised on a scale of years — DTK 1:50.000 has not been
 * updated since 2017 — so there is nothing to be gained by expiring sooner, and something to
 * lose: an expired tile is a blank square in a forest at 03:00.
 *
 * Note the service sends **no** `cache-control`, `etag` or `expires` headers at all, so there
 * is no revalidation to fall back on and no upstream freshness signal to respect. This number
 * is the only freshness policy there is.
 */
export const TILE_CACHE_MAX_AGE_SECONDS = 365 * 24 * 60 * 60

/**
 * Host serving the tiles. Used to build the Workbox URL pattern.
 */
export const TILE_HOST = 'api.dataforsyningen.dk'

/**
 * Query parameters stripped when computing a tile's **cache key**.
 *
 * The request still goes out with them; this only affects how the response is looked up, so
 * that two requests differing solely in these end up on one entry.
 *
 * - `token` — the Dataforsyningen quota key. It is delivered at runtime and can be rotated
 *   or differ per environment. Leaving it in the key would make a rotation silently miss a
 *   cache of several hundred megabytes, which is the worst possible moment for a full
 *   re-download.
 * - `_retry` — appended by the map's tile-retry logic to force an `<img>` reload
 *   (`EventMap.vue`). Without stripping it, a tile that failed once and succeeded on retry
 *   would be stored under `…&_retry=1` and never found again by its normal URL.
 *
 * **This list is duplicated as a literal in `vite.config.ts` and the two must stay in
 * sync.** That is not an oversight: Workbox's generateSW mode stringifies the
 * `cacheKeyWillBeUsed` callback into `sw.js` verbatim instead of bundling it, so the
 * callback cannot reference this constant — doing so builds cleanly and then throws
 * `ReferenceError` in the worker on every tile request. The constant is kept here so
 * in-app code (readiness view, purge) has one place to read, and so the reasoning above
 * lives somewhere other than a build file.
 */
export const TILE_CACHE_KEY_IGNORED_PARAMS = ['token', '_retry']
