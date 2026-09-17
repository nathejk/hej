# 315 — Offline datasets and split thumbnail/full media caching

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-18
**Completed:** 2026-09-18

## Description

PRD 019 §8. Two related pieces of offline plumbing.

**Dataset registration.** A `glimt` entry in `vue/src/config/offline.ts`
(`kind: 'cache-api'`, `sensitive: true`) with a budget, placed in the priority array so it is
evicted before `track` (unrecoverable) and `directory`. The **outbox must never be evicted**
while it holds unsent media (task 314).

**Two Workbox routes, two budgets.** Thumbnails and full media are cached separately. The
post-race browse pulls thousands of thumbnails and a handful of full images; under one budget
the thumbnails that make the grid usable get evicted by the full-size media that does not.
Thumbnails are small, numerous and worth keeping.

Also: add the public page path to `navigateFallbackDenylist` in `vue/vite.config.ts`, exactly
as `/desktop\.html$` already is — otherwise Workbox's navigation fallback serves the app shell
to an installed member following a public link, a silent and confusing failure.

**Watch the `generateSW` trap:** Workbox stringifies config functions into `sw.js`, so any
module-scope identifier used inside one becomes an undefined free variable at runtime. Use
literals.

## Acceptance Criteria

- [x] `glimt` dataset in `config/offline.ts` with a budget and a sensible eviction position
- [x] Separate `runtimeCaching` routes and budgets for thumbnails and full media
- [x] Outbox excluded from eviction while it holds unsent media
- [x] Public page path in `navigateFallbackDenylist`
- [x] No module-scope identifiers inside Workbox config functions
- [x] `npm run build` succeeds and the generated `sw.js` contains both routes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-18 — Picked up.
- 2026-09-18 — **Two caches, and the reason is worth keeping.** `nathejk-glimt-thumbs-v1` (3,000
  entries ≈ 9 MB) and `nathejk-glimt-media-v1` (200 entries ≈ 6 MB), sized from the fixture's measured
  bytes (2,961 for a 320px thumbnail, 28–31 kB for a 1600px image — task 327). Under one route they
  would share an entry cap and one LRU list, so opening twenty photographs would evict several hundred
  grid tiles: the cheap, numerous, load-bearing things pushed out by the expensive, rare ones. The grid
  is what a thousand people scroll at the finish line.
- 2026-09-18 — **Route order is load-bearing.** The thumbnail route matches on `variant=thumb` and must
  be registered *first*, because Workbox takes the first matching route and the full-media pattern has
  no query constraint — registered the other way round, the thumbnail cache would never receive a
  single entry and nothing would fail. Verified against the built `sw.js` rather than assumed: both
  cache names are present, and the thumbs route appears at an earlier offset than the media route.
- 2026-09-18 — `CacheFirst` for both, on the same grounds as tiles: the bytes are immutable by
  construction (a different image is a different content hash) and the BFF already says so with
  `immutable` and a content-hash ETag (task 305). There is nothing to revalidate against, and SWR would
  re-request every visible tile on every scroll — exactly the cost this exists to avoid.
- 2026-09-18 — `cacheableResponse: { statuses: [200] }`, which matters more here than for tiles: the
  media endpoint answers **403** for "not shared with you" (task 305), and caching that would freeze an
  authorization decision on the device for a fortnight — a member whose group changes mid-event would
  keep seeing the refusal. Not `purgeOnQuotaError`, for the reason the tile cache gives.
- 2026-09-18 — Placed **below portraits, above tiles** in the eviction order, with a test asserting it.
  A portrait is a safety feature — recognising the samarit coming to help you, in the dark — while a
  glimt is a memory; if one has to go it is not the safety one. Above tiles because everything is above
  tiles: they are ~99% of the bytes, so protecting 20 MB costs a rounding error of the map.
- 2026-09-18 — The evictor for this dataset is **composed**, and that is the interesting part: it takes
  full-size media first and only falls through to thumbnails for the remainder. Freeing 6 MB of
  viewer cache is nearly free; freeing the same from the grid costs two thousand tiles. Two tests — one
  that the thumbnails survive when the media cache alone suffices, one that it falls through when it
  does not.
- 2026-09-18 — **Closed the loop task 313 could only flag.** `purgeGlimtData()` drops the feed *and*
  both image caches when the server-issued deadline has passed, called from
  `registerOfflineDatasets()` on every launch. That launch check is the only thing that ever runs on a
  device which has not reopened the app since the event — and what it is deleting there is photographs
  of participants, not a list of names.
- 2026-09-18 — Added both caches to `OWN_CACHE_NAMES`. Easy to miss and visibly wrong if missed: the
  shell measurement excludes our own caches, so without this "the app itself" would appear to need
  20 MB.
- 2026-09-18 — Reported as **one dataset from two caches**. The split is a caching decision, not
  something to ask a participant to understand — the readiness view offers to clear "Glimt", not "Glimt
  thumbnails".
- 2026-09-18 — `navigateFallbackDenylist` now carries `/^\/offentligt\//`. Without it Workbox's
  navigation fallback serves the app shell to an installed member following a public link — a silent,
  confusing failure, and the trap `/desktop.html` already had an entry for. Verified in the built
  `sw.js`.
- 2026-09-18 — The `generateSW` stringification trap was **avoided by not needing it**: neither route
  has a `plugins`/`cacheKeyWillBeUsed` callback, so there is no function body to stringify and no
  module-scope identifier to become an undefined free variable. Every value used (`cacheName`,
  `maxEntries`, `urlPattern`) is outside a function body and inlined at build time.
- 2026-09-18 — One existing test needed updating rather than fixing: `browserEvictors` asserted "tiles
  and portraits, and nothing else". That expectation was correct when written and is now wrong; broadened
  to "the recoverable caches" with the Glimt ordering tested separately.
- 2026-09-18 — ✅ All criteria met. 837 tests, `type-check` and `build` clean.

### Note on the outbox criterion

"Outbox excluded from eviction while it holds unsent media" is satisfied **by construction rather than
by a rule**: the outbox is IndexedDB under `hej-glimt`, and `browserEvictors` only ever offers Cache API
buckets. There is no code path that could evict it, which is a better guarantee than a flag saying not
to — and it is the same reason the position track needs no protection beyond `unrecoverable`.

Worth knowing if the outbox ever gets a `budgetBytes` line of its own: it would then need
`unrecoverable: true`, because unsent media is precisely data that cannot be re-fetched.

### ⚠️ Not verified, and cannot be from here

The routes are in the built `sw.js`, but **no service worker has actually served a Glimt image from
them**. Two things a device would show that a build cannot: whether the `variant=thumb` pattern really
matches what the browser requests (Workbox matches on `url.href`, and a query-string pattern is the
kind of thing that works in a regex tester and not in a worker), and whether the caches fill and evict
as planned under real quota pressure.

The check is cheap once running: open the feed, then look for both `nathejk-glimt-*` caches in
DevTools › Application › Cache Storage, with thumbnails far outnumbering full images.
