# 313 — glimt.store.ts — feed state on the contacts-store pattern

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `glimt.store.ts` with `hydrate`, `fetch`, `refreshIfStale`, `persist`
- [ ] Profile-scoped, schema-versioned key; dropped on profile switch
- [ ] Runtime guard rejects a malformed stored payload without throwing
- [ ] `fetch` replaces state wholesale
- [ ] Wired to `useFreshnessLoop`
- [ ] 403 handled as "this role has no feed" rather than an error banner
- [ ] Unit tests for hydrate/guard/replace/expiry
- [ ] `npm run test:unit` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
