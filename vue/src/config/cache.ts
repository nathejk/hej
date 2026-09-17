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
 * Zoom tiers for the bulk race-area download (task 087).
 *
 * Two tiers because they are two different offers, not one download with a slider. Measured for the 2026
 * race area (428 km², topo + aerial):
 *
 *  - **orientation, z12–14** — 404 tiles, ~56 MB. The view a lost participant actually needs: where the
 *    area is, which way the roads run. Cheap enough to accept without much thought.
 *  - **detail, z15–16** — 4,887 tiles, ~268 MB. Refinement on top, and five times the bytes.
 *
 * z16 is the ceiling and not a candidate for trimming: DTK25 is a 508 DPI 1:25.000 product whose native
 * resolution is 1.25 m/px, which *is* z16 (1.34 m/px). z15 halves the source map's resolution, and z17 is
 * the same cartography upsampled — more bytes, no more information, which is why its tiles shrink.
 */
export const TILE_TIERS = {
  orientation: [12, 13, 14],
  detail: [15, 16],
} as const

export type TileTier = keyof typeof TILE_TIERS

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

/**
 * Cache holding Glimt **thumbnails** (`/api/glimt/items/{id}/media/{n}?variant=thumb`).
 *
 * # Why thumbnails and full media get separate caches
 *
 * This is the one decision in this file that PRD 019 asks for by name (§8), and it is worth stating
 * why rather than treating it as tidiness. The post-race browse (PRD 019 §0a.3) pulls **thumbnails by
 * the thousand** — a hold's collection is a grid — and full-size media a handful at a time, when
 * somebody opens one. Under a single Workbox route those share one entry cap and one LRU list, so
 * opening twenty photographs evicts several hundred grid tiles: the cheap, numerous, load-bearing
 * things are pushed out by the expensive, rare ones.
 *
 * Two caches means the grid keeps working while the viewer's cache churns, which is the behaviour the
 * finish line needs — a thousand people on the worst network of the weekend, and the grid is what
 * they scroll.
 */
export const GLIMT_THUMB_CACHE_NAME = 'nathejk-glimt-thumbs-v1'

/**
 * Maximum Glimt thumbnails retained.
 *
 * ~3 kB each at 320px (measured 2,961 bytes on the fixture, task 327). 3,000 entries is ~9 MB and
 * covers an entire event's worth of grid tiles several times over — a busy year might produce a few
 * hundred glimt with up to ten items each.
 *
 * Generous on purpose, for the reason the tile cap is: eviction here is not free. A thumbnail
 * discarded at the finish line is re-fetched over a congested network, and the grid is the surface
 * being scrolled at that exact moment.
 */
export const GLIMT_THUMB_CACHE_MAX_ENTRIES = 3_000

/**
 * Cache holding Glimt **full-size media**.
 *
 * Small on purpose — see `GLIMT_MEDIA_CACHE_MAX_ENTRIES`.
 */
export const GLIMT_MEDIA_CACHE_NAME = 'nathejk-glimt-media-v1'

/**
 * Maximum full-size Glimt media retained.
 *
 * ~30 kB each at 1600px (measured 28–31 kB, task 327), so 200 entries is ~6 MB.
 *
 * Deliberately a *small* cache. Its job is only to make going back to a photograph you just looked at
 * instant — swiping through a carousel, closing the viewer and reopening it. It is not trying to hold
 * an event's worth of full-size images, and if it did it would compete with the thumbnails that make
 * the grid usable.
 */
export const GLIMT_MEDIA_CACHE_MAX_ENTRIES = 200

/**
 * How long cached Glimt media is kept, in seconds. Fourteen days.
 *
 * Matched to `PORTRAIT_CACHE_MAX_AGE_SECONDS` and for the same reason, which applies more strongly
 * here: these are photographs of participants, and a device that never reopens the app after the
 * event must drop them on its own. That is the dormant-device case no purge can reach (PRD 009
 * §11.5), and it is why this is short rather than matched to the server's 90-day retention.
 *
 * Note the feed metadata carries a **server-issued** `expiresAt` derived from `GLIMT_RETENTION`
 * (task 313), and the store raises `expired` when it passes so these caches can be dropped in the
 * same beat. This constant is the belt to that braces: it bounds the bytes even if the app is never
 * opened again to run the purge.
 */
export const GLIMT_MEDIA_CACHE_MAX_AGE_SECONDS = 14 * 24 * 60 * 60

// ---------------------------------------------------------------------------
// Which URLs each runtime cache claims.
//
// # Why these are values here rather than literals in vite.config.ts
//
// PRD 007's single most important invariant is that **no spejder record is ever written to a
// device** (task 170). The patrol lookup is the only path by which a spejder's details are
// reachable, and it stays uncached by three separate means: `Cache-Control: no-store` from the BFF,
// never being declared a sync dataset, and **not being matched by any rule below**.
//
// That third one used to rest on nobody noticing: the rules lived as literals inside the build
// config, where no test could reach them, so the invariant held only because the existing patterns
// happen not to match. A well-meant `/^\/api\//` rule, or loosening the portrait matcher from
// `people` to `contacts/.*`, would have silently begun caching ~557 minors' faces on every crew
// device with nothing failing.
//
// Exported as values so `patrolLookupNeverCached.spec.ts` can run the **real** matchers against the
// **real** lookup URLs, rather than grepping for a string. A grep proves the current spelling; this
// proves the behaviour.
//
// # Safe to reference from `urlPattern`, unlike a function body
//
// Workbox's generateSW *stringifies* function bodies into `sw.js`, so an identifier from module
// scope becomes an undefined free variable in the worker — see the note on
// TILE_CACHE_KEY_IGNORED_PARAMS. `urlPattern` is different: it is evaluated at build time and its
// value inlined, so importing these is safe. `PORTRAIT_URL_PATTERN` is itself a function and is
// stringified, which is fine because its body closes over nothing but its own parameters. Keep it
// that way.

/** Dataforsyningen map tiles, by host — the layers live on several endpoints of one host. */
export const TILE_URL_PATTERN = new RegExp(`^https://${TILE_HOST.replace(/\./g, '\\.')}/`)

/**
 * Directory portrait thumbnails.
 *
 * **`people`, and anchored at both ends.** Both halves are load-bearing: the patrol lookup serves
 * faces from `/api/contacts/patrols/{number}/photo/{personId}`, so a matcher spelled
 * `/api/contacts/.*\/photo` — or one without the `$` — would claim minors' portraits from the one
 * endpoint that must never be cached. This is the rule most likely to be widened by somebody
 * reasonably thinking the two photo endpoints are the same thing. They are not.
 */
export const PORTRAIT_URL_PATTERN = ({ url, sameOrigin }: { url: URL; sameOrigin: boolean }) =>
  sameOrigin && /^\/api\/contacts\/people\/[^/]+\/photo$/.test(url.pathname)

/** Glimt grid thumbnails. Must be registered before the full-media rule — first match wins. */
export const GLIMT_THUMB_URL_PATTERN = /\/api\/glimt\/items\/[^/]+\/media\/\d+\?.*variant=thumb/

/** Glimt full-size media, as the viewer opens them. */
export const GLIMT_MEDIA_URL_PATTERN = /\/api\/glimt\/items\/[^/]+\/media\/\d+/

/**
 * Every runtime-caching matcher, for the guard that asserts none of them claims a patrol lookup.
 *
 * A list rather than the test importing four names, so that **adding a fifth cache without adding
 * it here** is the only way to escape the guard — and that is a visible omission in a reviewed
 * diff, where a new `urlPattern` in the build config is not.
 */
export const RUNTIME_CACHE_MATCHERS: Array<{
  name: string
  pattern: RegExp | ((ctx: { url: URL; sameOrigin: boolean }) => boolean)
}> = [
  { name: 'map tiles', pattern: TILE_URL_PATTERN },
  { name: 'directory portraits', pattern: PORTRAIT_URL_PATTERN },
  { name: 'glimt thumbnails', pattern: GLIMT_THUMB_URL_PATTERN },
  { name: 'glimt media', pattern: GLIMT_MEDIA_URL_PATTERN },
]
