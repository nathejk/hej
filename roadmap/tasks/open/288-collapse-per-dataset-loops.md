# 288 — Collapse the per-dataset loops into the sync loop

**Status:** open
**Priority:** high
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `useContactsFreshness` removed (or reduced to nothing but a re-export) and its
      call site in `ContactsView.vue` updated.
- [ ] `useQuietPrefetch`'s freshness loop removed; `App.vue` registers `useSyncLoop`
      only.
- [ ] Any non-freshness responsibility in `prefetch.ts` (first-run warm-up, tile
      concerns) either kept deliberately or moved with a comment saying where.
- [ ] `MapsView.vue`'s `onMounted` scan fetch removed.
- [ ] No dataset is fetched by two mechanisms; exactly one app-level loop remains.
- [ ] A pane nothing has opened does not trigger a payload fetch.
- [ ] `useContactsFreshness.spec.ts` and `prefetch.spec.ts` updated or replaced rather
      than deleted — the behaviours they assert still matter, they just have a new
      owner.
- [ ] Manual check: opening contacts, the map and profile issues one `/api/sync` per
      foreground and no duplicate payload requests.
- [ ] `npm run type-check` and the unit suite green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Depends on tasks 286 and 287.
