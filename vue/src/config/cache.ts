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
 *
 * **Who owns eviction for this cache** (decided in task 186, since two mechanisms now touch it):
 * Workbox's `expiration` owns *routine* trimming — it enforces this entry cap on every write,
 * least-recently-used, and is the only thing that runs inside the service worker.
 * `helpers/offline/eviction.ts` owns *quota-pressure* eviction — it runs in the app, only after a
 * write has actually failed, and deletes by descending zoom. They do not conflict because they
 * answer different questions ("is this cache too long?" versus "is the origin full?"), but neither
 * may be given the other's job: an LRU pass under quota pressure would discard whatever the user
 * last looked away from, which in a forest is the area they are walking into.
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

/**
 * Cache holding portrait thumbnails (`/api/contacts/people/{id}/photo`).
 *
 * **Its own cache, added in task 192.** Before that, portraits relied on the browser's HTTP cache
 * via `Cache-Control: private, max-age=3600` on the response — which works for display and is
 * useless for everything PRD 009 needs: an HTTP cache cannot be measured, cannot be shown in the
 * readiness view, cannot be evicted in a priority order, and cannot be purged after the event.
 * PRD 009 §8 asks for "a route and expiry policy per binary dataset" for exactly this reason.
 *
 * Note this makes portrait bytes *durable* rather than incidental, which is a privacy change as
 * much as a storage one — hence the short expiry below and the post-event purge in task 193.
 */
export const PORTRAIT_CACHE_NAME = 'nathejk-portraits-v1'

/**
 * Maximum portrait thumbnails retained.
 *
 * ~4.5 kB each at `thumb256` (task 104), and the largest cached population is ~151 people (task
 * 078). 1,000 covers every role several times over, including a member browsing a directory that
 * changes during the event, at a ceiling of ~4.5 MB.
 */
export const PORTRAIT_CACHE_MAX_ENTRIES = 1_000

/**
 * How long a cached portrait is kept, in seconds. Two weeks.
 *
 * **Approved by the maintainer 2026-09-01**, and deliberately equal to the BFF's
 * `CACHED_DIRECTORY_TTL`: the index and the faces expire together, because a directory of names
 * with no photographs and a set of photographs with no names are both worse than neither. If one
 * changes, change both in the same commit — otherwise half a purge looks like a whole one.
 *
 * Long enough to cover the run-up plus the race, so a participant who prepares a fortnight early
 * still has faces on the night. Short enough that a device which never reopens the app after the
 * event drops them on its own — the dormant-device case where no purge can run (PRD 009 §11.5).
 * A content hash (`?v=`) is already in the URL, so staleness is not the reason for an expiry here;
 * not keeping photographs of people indefinitely is.
 */
export const PORTRAIT_CACHE_MAX_AGE_SECONDS = 14 * 24 * 60 * 60
