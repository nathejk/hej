# 192 — Reconcile the existing caches with the agreed budget

**Status:** done
**Priority:** medium
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

PRD 009 §10. Four caches already ship, each written before the shared policy existed:

| dataset | storage | where |
|---|---|---|
| map tiles | Cache API (Workbox runtime route) | `vue/vite.config.ts`, `src/config/cache.ts` |
| contacts directory | `localStorage` | `vue/src/stores/contacts.store.ts` |
| location track | IndexedDB | `vue/src/helpers/trackDb.ts` |
| app shell | Workbox precache | `vue/vite.config.ts` |

**No rewrites.** PRD 009 §4 explicitly does not migrate them onto a common primitive: tiles
belong in the Cache API because a service worker serves them, the track belongs in IndexedDB
because it is large and unrecoverable. What each owes is only: a declared size, a rank in task
183's order, its `unrecoverable` flag, a server-issued expiry where the data is sensitive, and
reporting into `offline.store` (task 184) so it appears in the readiness view.

This is the task that decides whether PRD 009 actually took effect or just produced a document.
Without it the order is unobserved by every existing cache, which PRD 009 §8 names as the honest
cost of having cut the registry.

## Acceptance Criteria

- [x] All four datasets appear in task 183's declaration with real sizes. *Shell corrected from a
      guessed 5 MB to 2 MB against a measured 738 kB precache; portraits aligned to the new cache's
      own entry cap so the two cannot disagree about "full".*
- [x] All four report status into `offline.store` and appear in the readiness view (task 187).
- [x] The track is flagged `unrecoverable`; the other three are not.
- [x] The tile cache's Workbox `expiration` and task 186's eviction policy do not both claim
      ownership of the same bucket — split and documented in `config/cache.ts` (done in 186).
- [ ] The directory carries a **server-issued** expiry — **handed to task 193**, which owns the
      server side of it. Nothing here fakes one client-side, because a client-computed TTL is
      exactly what a wrong device clock defeats.
- [x] The patrol lookup is **absent** from all of it — it is not a dataset in
      `config/offline.ts`, has no reporter, no handler and no cache route.
- [x] The directory's **metadata index is independently usable when images are absent** — they are
      now genuinely separate caches, and a test asserts evicting portraits leaves the directory
      complete.
- [x] Bulk image sync stays pre-race and user-initiated; metadata deltas may run during the race.
      *Unchanged by this task: portraits are still fetched by the rows that display them, and the
      only download button offered is the directory's manifest.*
- [x] Sizing handed to the budget.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — **Task 161 folded in and closed as superseded.** It covered the same work for
  two of these four datasets; doing it separately would have meant two passes over one
  declaration. Its criteria are carried above verbatim.
- 2026-09-01 — Picked up. Plan: measure each cache, register reporters and handlers from App.vue, and give portraits their own Workbox route so they are separately measurable at all.
- 2026-09-01 — **Finding: portraits were not a cache at all.** They were being kept only by the
  browser's HTTP cache, via `Cache-Control: private, max-age=3600` on the response. That is enough
  to draw a list and useless for everything PRD 009 needs: an HTTP cache cannot be measured, cannot
  appear in the readiness view, cannot be evicted in a priority order and cannot be purged after the
  event. So they now have their own Workbox route and named bucket — which is also a privacy
  change, since portrait bytes become durable rather than incidental. Hence a 14-day expiry: long
  enough for a participant who prepares a fortnight early, short enough that a phone which never
  reopens the app drops the faces on its own, which is the dormant-device case no purge can reach.
- 2026-09-01 — **`cacheableResponse: [200]` matters more for portraits than for tiles.** PRD 007
  makes that endpoint return an indistinguishable 403/404 for "not allowed" and "no photo" — caching
  either would freeze an authorization decision on the device for a fortnight, so a member whose
  role changed mid-event would keep seeing the refusal.
- 2026-09-01 — **Reporting is pulled, not pushed.** Three of the four caches cannot tell the page
  when they change: the tile cache and the precache are written by the service worker, which does
  not notify anyone. So they are measured at startup and again when the readiness view is opened,
  rather than pretending to a live count. The directory still reports on write, because it can.
- 2026-09-01 — **Tiles are never reported `complete`.** They arrive as the map is browsed (task
  087's cheap half), so a non-empty cache means "some of the map", never "the map". Anything else
  would put "Klar" next to a map with holes in it. The shell, by contrast, is genuinely
  all-or-nothing — if the precache had not installed, the app would not be running.
- 2026-09-01 — **The contacts store now retries a failed write after making room.** This is the
  first place the priority order does real work: the directory ranks *above* map tiles, so a full
  origin should cost the user some of the map rather than the phone numbers of the people they work
  with. One retry only — a second failure means something other than tiles is filling the origin,
  and looping would just delay saying so.
- 2026-09-01 — `browserEvictors` went in its own file rather than `reporters.ts`: the contacts store
  needs it, `reporters.ts` needs the contacts store, and that is a module cycle for no reason.
- 2026-09-01 — **Handlers only where there is something honest to offer.** Directory gets sync and
  clear; tiles get clear only, because there is no bulk download yet (task 087) and a "hent nu"
  that re-fetched whatever the map last showed would be a button that appears to work and does
  almost nothing. Shell, portraits and track get none.
- 2026-09-01 — Verified in the **generated worker**, not just the config: `dist/sw.js` contains the
  portrait route with its matcher function intact and `cacheName:"nathejk-portraits-v1"`. That check
  exists because task 087 shipped a route that built cleanly and threw on every request.
- 2026-09-01 — ✅ Criteria complete except the server-issued expiry, handed to task 193 which owns
  the server side. 18 new tests (measure 7, reporters 11); suite 298 across 24 files; `type-check`
  and `build` clean.
- 2026-09-01 — Done. The readiness view now says something real: five rows with sizes, counts and
  last-sync times, and a working "Hent nu" for the directory.
