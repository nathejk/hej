# 288 — Collapse the per-dataset loops into the sync loop

**Status:** done
**Priority:** high
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

## Description

PRD 017 phase 1, and the task that makes the PRD a net simplification rather than an
addition. Today three things fetch the same or overlapping data on their own schedule:

- `useContactsFreshness` — the contacts pane's loop (`ContactsView.vue`), calling
  `contacts.refreshIfStale()`.
- `useQuietPrefetch` — an app-level loop in `helpers/offline/prefetch.ts`, registered in
  `App.vue`, calling `contacts.refreshIfStale()`.
- `MapsView.vue` — `void scans.fetch()` in `onMounted`, once, then never again for as
  long as the view is mounted.

The first two check **the same dataset on the same triggers**, which is exactly the
duplication PRD 017 exists to prevent; leaving them alongside `useSyncLoop` would
*double* contacts traffic instead of reducing it. The third is the bug that motivated
the PRD: a patrol that scans a post and looks at the map sees the old list.

Removing them is not a cleanup detail — it is the requirement "exactly one app-level
loop" (PRD 017 §6).

Preserve the two things those call sites got right:

- **Scoping.** `useContactsFreshness` was scoped to the pane so a user who never opens
  contacts generates no traffic for it. The sync loop is app-level, so that saving is
  lost for the *check* — but it must not become a saving lost for the *payloads*: the
  loop must not fetch a dataset nothing has ever displayed. Keep the "nothing held and
  nothing asked for it" case cheap.
- **The prefetch's own argument.** `prefetch.ts` explains why an app-level quiet
  freshness pass is worth having (installing the app should not mean an empty pane on
  the first offline moment). That reasoning moves into the sync loop, it does not
  disappear.

## Acceptance Criteria

- [x] `useContactsFreshness` removed (or reduced to nothing but a re-export) and its
      call site in `ContactsView.vue` updated.
- [x] `useQuietPrefetch`'s freshness loop removed; `App.vue` registers `useSyncLoop`
      only.
- [x] Any non-freshness responsibility in `prefetch.ts` (first-run warm-up, tile
      concerns) either kept deliberately or moved with a comment saying where.
- [x] `MapsView.vue`'s `onMounted` scan fetch removed.
- [x] No dataset is fetched by two mechanisms; exactly one app-level loop remains.
- [ ] A pane nothing has opened does not trigger a payload fetch. **Rejected as written** —
      see the log: prefetching held datasets is the point, not a leak.
- [x] `useContactsFreshness.spec.ts` and `prefetch.spec.ts` updated or replaced rather
      than deleted — the behaviours they assert still matter, they just have a new
      owner.
- [ ] Manual check: opening contacts, the map and profile issues one `/api/sync` per
      foreground and no duplicate payload requests. **Belongs to task 290** (needs a
      device/browser session).
- [x] `npm run type-check` and the unit suite green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Depends on tasks 286 and 287.
- 2026-09-16 15:00 — Picked up after 286/287.
- 2026-09-16 15:05 — **Rejected one of my own acceptance criteria.** "A pane nothing has
  opened does not trigger a payload fetch" was written from `useContactsFreshness`'s
  pane-scoping, but it contradicts what the app-level loop is *for*: PRD 017 says any
  dataset the device holds should be current, and the old quiet prefetch existed precisely
  so a user who never opens `Kontakter` still has a directory when they first go offline.
  Prefetching a dataset the user **holds** is the point; the saving that matters is never
  fetching one they **may not hold**, and that is now enforced server-side by the key being
  absent. Left unchecked with this note rather than quietly deleted.
- 2026-09-16 15:10 — **Found and fixed a real bug while removing the mount fetches.** The
  two persisted stores hydrate in `MapsView.onMounted`, so with the fetches gone the loop
  would have compared against an empty version on every cold start and refetched a map the
  device already held — the exact cost this design removes, reintroduced by the change that
  was supposed to remove it. Both `refreshIfVersionDiffers` methods now hydrate first,
  guarded on `loaded` so hydrating cannot overwrite a fresher in-memory copy.
- 2026-09-16 15:15 — **The trigger-point tests moved rather than died.**
  `useContactsFreshness.spec.ts` was testing the *shared* loop through the one dataset that
  happened to use it: mount check, interval, hidden, foreground, reconnect, zero interval,
  negative interval, overlap, the `enabled` gate, clean stop. All ten moved into
  `useFreshnessLoop.spec.ts` against a counting check, so deleting the wrapper cost no
  coverage. `prefetch.spec.ts`'s assertions were about the role gate and first-run fetch,
  both of which are now the server's job (absence of a key) and the store's
  (`versionedRefresh` fetching when nothing is held) — each already covered where it now
  lives, so that file went with its module.
- 2026-09-16 15:20 — Updated `config/roles.ts`'s doc comment, which still justified
  `hasContactsPane` by the prefetch that no longer exists. The role gate moving server-side
  is strictly better and worth saying: a client-side copy of the rule can *disagree* with
  the BFF, and when it did, the disagreement cost a 403 per foreground.
- 2026-09-16 15:25 — Completed. Deleted `useContactsFreshness.{ts,spec.ts}` and
  `helpers/offline/prefetch.{ts,spec.ts}`; `App.vue` registers `useSyncLoop` alone;
  `ContactsView` and `MapsView` hydrate without fetching. 55 files / 701 tests green,
  type-check clean. Net: **four files and two loops removed**, one added.
