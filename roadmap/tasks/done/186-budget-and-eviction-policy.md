# 186 — Budget enforcement and priority-ordered eviction

**Status:** done
**Priority:** high
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

PRD 009 §6. Make task 183's order operative: when the origin approaches its ceiling, what
gets dropped is decided by rank, not by whichever cache happened to write last — and never
by discarding a whole cache.

Two rules do most of the work:

1. **`QuotaExceededError` is expected, not exceptional.** A failed write leaves everything
   already held intact and working. A full cache that cannot grow is much better than an empty
   one. `vue/vite.config.ts` already encodes this by refusing Workbox's `purgeOnQuotaError`,
   with the reasoning in a comment — generalise it, do not re-derive it.
2. **Evict from the bottom of the order.** Tiles first, highest zoom first; unrecoverable data
   never.

**The hard part is not the algorithm, it is reach.** There is no registry (PRD 009 §4), so this
policy has to act on three caches it does not own: a Workbox-managed Cache API bucket,
`localStorage`, and IndexedDB. Expect to need a per-kind adapter, and expect the Cache API one
to be the awkward one, since Workbox's own `expiration` plugin also evicts on its own terms.
PRD 009 §8 names this as the honest cost of cutting the registry; task 195's quota test is what
keeps it honest.

## Acceptance Criteria

- [x] `QuotaExceededError` is caught and handled on **both** the Cache API and IndexedDB paths;
      nothing purges a whole cache to recover. *`isQuotaExceeded` covers the modern and legacy
      names; the IndexedDB side already had `TrackStorageFullError`, and its data is
      unrecoverable so the response there is to stop and say so, never to discard.*
- [x] Eviction walks task 183's order upward from the bottom and refuses to touch anything
      flagged `unrecoverable` — asserted by test, not by convention. *Tested by offering an
      evictor for the track and checking it is never even asked.*
- [x] Tile eviction removes **highest zoom first**.
- [x] What was dropped is recorded in `offline.store` so the readiness view can say so.
- [x] Workbox's own expiration for the tile cache does not fight this policy — ownership split
      and documented in `config/cache.ts`.
- [x] No behaviour depends on `navigator.connection`.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — Picked up. Plan: helpers/offline/eviction.ts, per-storage-kind adapters, quota handling, tile zoom derived from the WMS bbox.
- 2026-09-01 — **Finding that changes the design: the map is WMS, not XYZ/WMTS.** A WMS request
  identifies its tile by `bbox` and pixel size — there is **no zoom anywhere in the URL** — so
  "evict the highest zoom first" cannot be done by matching a path segment. Derived it instead:
  Leaflet's default CRS is EPSG:3857, whose world is 40,075,016.686 m wide and 2^z tiles across, so
  `z = log2(WORLD / bboxWidth)`. Own module (`tileZoom.ts`) with its own tests, because it is the
  one piece of arithmetic here that is quietly wrong if the CRS assumption ever changes.
  Rounded, not floored: a server snapping to its own grid puts the value a hair off an integer and
  flooring would reclassify a whole layer one level down.
- 2026-09-01 — **Unknown-zoom entries are evicted last, not first.** An entry this code cannot
  parse is more likely to be something it does not understand than a tile we meant to discard — so
  deleting those first would make a URL-shape change *look* like working eviction while quietly
  emptying the cache. Same reasoning admits a stray z17 to the end of the declared order rather
  than leaving it undeletable.
- 2026-09-01 — **Sizes are estimated, not measured.** Reading every body to count bytes means
  decoding several hundred megabytes on a 5,000-tile cache — spending a phone's battery in a
  forest to be precise about a number only used for "have I freed enough yet". `content-length`
  when present, measured per-zoom figures from task 087 otherwise (Dataforsyningen sends no cache
  headers at all, so the fallback is the normal path).
- 2026-09-01 — **Freeing a generous 20 MB per pass rather than exactly what failed.** Freeing the
  minimum would trigger a full cache scan on every subsequent tile write — hundreds of passes
  while a participant pans a map.
- 2026-09-01 — **Ownership split with Workbox, documented in `config/cache.ts`:** Workbox's
  `expiration` owns routine LRU trimming inside the worker; this policy owns quota-pressure
  eviction in the app, by descending zoom. They answer different questions and neither may take
  the other's job — an LRU pass under quota pressure discards whatever the user last looked away
  from, which in a forest is the area they are walking into.
- 2026-09-01 — **No timers, no size watching.** Eviction runs only when a write has actually
  failed. "Nearly full" from `navigator.storage.estimate()` is a number iOS rounds and pads, and
  acting on it means evicting a participant's map because of an estimate.
- 2026-09-01 — ✅ All criteria complete. 25 new tests (eviction 19, tileZoom 6) plus 2 in the
  store; suite 267 passing across 21 files; `type-check` clean.
- 2026-09-01 — Done. The evictors are constructed by whoever owns the storage — task 192 wires
  the real ones. Until then `reclaimSpace` has no callers, which is deliberate: the policy is
  ready and inert rather than half-wired.
