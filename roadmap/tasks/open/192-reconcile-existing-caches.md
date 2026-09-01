# 192 — Reconcile the existing caches with the agreed budget

**Status:** open
**Priority:** medium
**Created:** 2026-09-01

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

- [ ] All four datasets appear in task 183's declaration with real sizes.
- [ ] All four report status into `offline.store` and appear in the readiness view (task 187).
- [ ] The track is flagged `unrecoverable`; the other three are not.
- [ ] The tile cache's Workbox `expiration` and task 186's eviction policy do not both claim
      ownership of the same bucket — one does, and which is documented.
- [ ] The directory carries a **server-issued** expiry (with task 193).
- [ ] The patrol lookup is **absent** from all of it, with a comment saying why — it is
      `no-store` by decision and caching it would undo PRD 007's central privacy property
      (tasks 157, 170).

Carried from **task 161**, folded in here 2026-09-01:

- [ ] The directory's **metadata index is independently usable when images are absent or
      evicted** — search, groups and favourites keep working on names alone. This is what the
      index/binary separation is *for*, so it needs an actual test with the portrait cache
      emptied, not just a separate storage location.
- [ ] Bulk image sync stays pre-race and user-initiated; **metadata deltas are exempt** and may
      run during the race on mobile data (PRD 007 §6, PRD 009 §6). "WiFi-only" is not
      implementable on iOS — the restriction is *user-initiated with a size estimate*.
- [ ] Sizing handed to the budget: ~151 banditter ≈ 0.7 MB, ~99 gøglere ≈ 0.4 MB, ~20 crew
      ≈ 0.1 MB at `thumb256` ≈ 4.5 KB (tasks 078, 104) — under ~1 MB for the largest role.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — **Task 161 folded in and closed as superseded.** It covered the same work for
  two of these four datasets; doing it separately would have meant two passes over one
  declaration. Its criteria are carried above verbatim.
