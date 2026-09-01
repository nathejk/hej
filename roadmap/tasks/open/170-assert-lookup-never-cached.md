# 170 — Assert nothing from a patrol lookup is cached

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

The single most important invariant in PRD 007: **no spejder record is ever written to a
device.**

An earlier draft had the patrol lookup working offline, which meant shipping ~557 spejder
thumbnails to every crew device — a deliberate relaxation of "scope the payload, not the
view", and the largest privacy cost in the design. The 2026-08-31 decision to leave the
lookup uncached removed it entirely. This task is what keeps it removed.

The threat is not malice, it is drift: a service worker route added later, a well-meaning
"add offline support to the lookup" change, or a generic PRD 009 dataset registration
would each silently undo it. Enforcement therefore has to be mechanical:

- `Cache-Control: no-store` on the lookup and its photo endpoint (task 157);
- the routes excluded from the service worker's runtime caching in `vite.config.ts` /
  `push-sw.js`;
- the lookup never declared as a cached dataset under PRD 009 (task 161);
- and a test that actually inspects storage after a lookup.

## Acceptance Criteria

- [ ] Test: perform a patrol lookup, then assert the Cache Storage API holds no entry
      whose URL matches the lookup routes.
- [ ] Test: after a lookup, no spejder id, name, number or image is present in
      localStorage, IndexedDB or any Pinia-persisted store.
- [ ] Test asserts `no-store` is present on both lookup responses.
- [ ] Service worker route config asserted to exclude the lookup paths.
- [ ] The tests are named so their purpose is obvious (`TestPatrolLookupIsNeverCached`
      or similar), and carry a comment explaining what breaks if they are deleted.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8.
