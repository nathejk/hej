# 313 — glimt.store.ts — feed state on the contacts-store pattern

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-18
**Completed:** 2026-09-18

## Description

PRD 019 §8. Follow `vue/src/stores/contacts.store.ts` rather than inventing a second pattern:

- schema-versioned, **profile-scoped** storage key (`helpers/profileStorage.ts`) — a bandit
  must not see the crew feed cached on the same device after a profile switch (PRD 012)
- runtime type guards on read, because anything from `localStorage` is untrusted
- server-issued `expiresAt` enforced in `hydrate()`
- a storage seam interface so tests run in `node`
- `refreshIfStale()` on the cheap `/api/glimt/version` probe
- **replace, never merge**, on fetch — so a delete, a hide or a purge is not decorative

Feed **metadata** goes in `local-storage` (small, must be droppable). Feed **media** is cached
by the service worker (task 315), not by this store.

Freshness uses the existing shared `useFreshnessLoop` composable — foreground, interval while
visible, and `online` — not a bespoke poller.

## Acceptance Criteria

- [x] `glimt.store.ts` with `hydrate`, `fetch`, `refreshIfVersionDiffers`, `persist`
- [x] Profile-scoped, schema-versioned key; dropped on profile switch
- [x] Runtime guard rejects a malformed stored payload without throwing
- [x] `fetch` replaces state wholesale
- [x] Wired to the sync loop — a `glimt` key in the dispatch table, **not** `useFreshnessLoop` directly
- [x] 403 handled as "this role has no feed" rather than an error banner
- [x] Unit tests for hydrate/guard/replace/expiry
- [x] `npm run test:unit` passes (763 tests), plus `type-check` and `build`

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-18 — Picked up. Read the skill, `contacts.store.ts`, `syncVersions.ts` and
  `useFreshnessLoop.ts` first. Two corrections to this task's own wording came out of that:
  - The method is **`refreshIfVersionDiffers(version)`**, not the `refreshIfStale()` the task and
    PRD §8 name. A store does not fetch its own version any more — `/api/sync` hands it one
    (task 292 retired the per-dataset endpoints, and `useFreshnessLoop.ts` says so at length).
  - The store hooks into **`useSyncLoop`'s dispatch table**, not `useFreshnessLoop`. The dispatch
    table is a `Record` over the `SyncDataset` union precisely so a dataset the server reports and
    the client forgot is a *type error* rather than a dataset that silently never refreshes — so
    adding `'glimt'` to the union was what forced the wiring to exist.
- 2026-09-18 — **Added `expires_at` to the feed response server-side.** The criterion asked for a
  server-issued deadline and there was none to consume. Derived from `glimtRetention` rather than a
  new knob: "a device may keep this for as long as the server would have kept it" needs no second
  number to keep in agreement, and shortening the server window shortens the client's in the same
  edit.
  The dormant-device argument is **stronger here than for contacts**, which is why it is worth the
  server change: what a phone that never reopens the app holds is not a list of names but
  **photographs of children**, cached by the service worker where the retention sweep cannot reach
  them. `expired` is raised as a flag so whoever owns the media cache drops the images in the same
  beat — a feed of captions with no pictures and a pile of pictures with no feed are both worse than
  neither, and the second is a pile of photographs nothing points at.
- 2026-09-18 — An unrecognised `audience` maps to **`group`, the narrowest**. A value this bundle
  does not understand must not be *rendered* as more widely shared than it is. The server remains
  the authority on who actually sees it; this only decides which chip to draw.
- 2026-09-18 — 503 is kept distinct from a generic failure, because the BFF distinguishes
  "unavailable" from "empty" on purpose (task 304) and the client must not collapse them: an empty
  feed is a legitimate state a device would cache, so showing one because the database is down would
  leave a phone displaying "nothing was shared tonight" for the rest of the event.
- 2026-09-18 — `glimtMediaUrl()` lives in the store rather than in a component, so there is one
  definition of the path and the `variant=thumb` decision sits next to the `hasThumb` flag that
  informs it. Tests assert the ordinal-addressed shape — no content hashes, since a hash in a
  *cached* payload would be a forwardable capability outliving every visibility check.
- 2026-09-18 — 🐞 Three tests failed on first run, all one cause: I called
  `new HttpError('forbidden', 403)` when the constructor is `(status, message)`. Worth noting because
  the failure was informative rather than annoying — the 403 and 503 branches were genuinely not
  being exercised, and a looser assertion would have let both paths ship untested.
- 2026-09-18 — ✅ All criteria met. 23 new tests; the full suite is 763 passing across 60 files;
  `npm run type-check` and `npm run build` both clean. Moving to done.

### Notes for the views (316, 317, 326)

- Import `glimtMediaUrl` rather than building paths; the grid wants `'thumb'` and the viewer
  `'full'`.
- `store.expired` means the media cache must be dropped too. That belongs to task 315, which owns
  the Cache API budget — do not paper over it in a view.
- `store.forbidden` is not an error state: render nothing, apologise for nothing.
- The feed is **metadata only**. Nothing in this store knows about bytes, and nothing should: the
  images are `<img>` tags the service worker caches.
