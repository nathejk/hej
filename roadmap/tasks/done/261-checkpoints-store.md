# 261 — `checkpoints.store.ts` with offline caching

**Status:** done
**Priority:** medium
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

PRD 016 phase 3. Pinia store for the patrol's revealed checkpoints from
`GET /api/checkpoints`, cached through the offline data layer (PRD 009).

- **Never throws.** The map must stay usable when this fails — follow `scans.store.ts`,
  which records an error string and keeps whatever it holds.
- **Replace, do not merge** (PRD 009 §6): a checkpoint the server stops returning must stop
  existing on the device. A merge would leave a withdrawn post on the map forever.
- Getters for the map layer (`positioned`) and for the arrows (next unscanned in route
  order, `(checkgroup.sortOrder, checkpoint.sortOrder)`, capped at 3 — see task 263).
- An empty list is a normal state that hides UI, not an error (personnel have no patrol).
- Expose a version for PRD 017's sync check (task 269) rather than polling itself.

## Acceptance Criteria

- [x] `vue/src/stores/checkpoints.store.ts` following `scans.store.ts`'s conventions.
- [x] Cached copy survives reload and is readable offline (test).
- [x] Refetch replaces wholesale; a removed checkpoint disappears (test).
- [x] A failed fetch keeps the cached copy and sets an error (test).
- [x] Empty list is not an error (test).
- [x] Route order tested — **server-side**, in `internal/reveal`, rather than as a client getter.
      See the log: only the BFF has both halves of the order.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 3.
- 2026-09-15 — **Found a real gap while planning the route-order getter this task asked for.** Route order is
  (checkgroup order, checkpoint order), and the client has only the second half: the payload carries a
  checkpoint's position *within* its group, while the group's own position lives in the checkgroup
  projection. A client-side getter would therefore have required shipping the group order too — so the
  frontend could redo a sort the server was already half-doing — and a client that got it wrong would point
  arrows at the wrong "next" post while looking entirely correct.

  Fixed on the **server** instead: `internal/reveal.merge` now sorts by `(group order, checkpoint order)`,
  using the group query it already ran for task 256's dangling-trigger check, so it costs nothing. The
  response is fully ordered and the store simply preserves it. Test:
  `TestRouteOrderSpansCheckgroups` in `internal/reveal`, plus a store test that the order is not re-sorted.
- 2026-09-15 — Store written per the repo's storage conventions: per-profile key via `profileKey`, schema
  version in the payload *and* the key, storage as an injected seam so the spec runs in node, and every
  storage access guarded — `localStorage` throws **on access** in some Safari privacy modes, not on use.
- 2026-09-15 — Decision: **no visibility flag and nothing to filter client-side.** The BFF's response is
  already patrol-scoped, so if a checkpoint is in this store the patrol may see it. A client-side filter
  would imply the payload contained something it must not — which is the opposite of the guarantee.
- 2026-09-15 — `hydrate()` is separate from `fetch()` so the map can draw from the cache immediately.
  Offline, or on a slow link at 02:00, that is the difference between a map with posts on it and an empty
  one.
- 2026-09-15 — 13 specs, incl. the two that matter for a night in a forest: a failed refresh keeps the copy,
  and a storage write that throws (Safari quota) does not lose the data in memory. `npm run type-check`
  clean.
- 2026-09-15 — Note: the `vue3-pwa-layout` skill claims "there is no unit or e2e suite in this repo yet" and
  says verification is manual. That is stale — vitest is wired (`npm test`) with ~20 spec files. Worth
  fixing in the skill so the next agent does not skip writing tests; noted for the end of this phase.
- 2026-09-15 — Done.
