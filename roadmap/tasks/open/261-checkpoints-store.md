# 261 — `checkpoints.store.ts` with offline caching

**Status:** open
**Priority:** medium
**Created:** 2026-09-15

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

- [ ] `vue/src/stores/checkpoints.store.ts` following `scans.store.ts`'s conventions.
- [ ] Cached copy survives reload and is readable offline (test).
- [ ] Refetch replaces wholesale; a removed checkpoint disappears (test).
- [ ] A failed fetch keeps the cached copy and sets an error (test).
- [ ] Empty list is not an error (test).
- [ ] Route-order getter tested, including ties.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 3.
