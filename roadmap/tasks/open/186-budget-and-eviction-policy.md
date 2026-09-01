# 186 — Budget enforcement and priority-ordered eviction

**Status:** open
**Priority:** high
**Created:** 2026-09-01

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

- [ ] `QuotaExceededError` is caught and handled on **both** the Cache API and IndexedDB paths;
      nothing purges a whole cache to recover.
- [ ] Eviction walks task 183's order upward from the bottom and refuses to touch anything
      flagged `unrecoverable` — asserted by test, not by convention.
- [ ] Tile eviction removes **highest zoom first** (z16 is 60% of the bytes and the least
      information per byte; z12–14 is orientation and must survive longest).
- [ ] What was dropped is recorded in `offline.store` so the readiness view can say so. Silent
      eviction is the failure mode PRD 009 §9 measures against.
- [ ] Workbox's own expiration for the tile cache does not fight this policy — one of them
      owns eviction for that bucket, and which one is documented.
- [ ] No behaviour depends on `navigator.connection`: it is unavailable in Safari (PRD 009 §8).

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
