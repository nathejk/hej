# 315 — Offline datasets and split thumbnail/full media caching

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §8. Two related pieces of offline plumbing.

**Dataset registration.** A `glimt` entry in `vue/src/config/offline.ts`
(`kind: 'cache-api'`, `sensitive: true`) with a budget, placed in the priority array so it is
evicted before `track` (unrecoverable) and `directory`. The **outbox must never be evicted**
while it holds unsent media (task 314).

**Two Workbox routes, two budgets.** Thumbnails and full media are cached separately. The
post-race browse pulls thousands of thumbnails and a handful of full images; under one budget
the thumbnails that make the grid usable get evicted by the full-size media that does not.
Thumbnails are small, numerous and worth keeping.

Also: add the public page path to `navigateFallbackDenylist` in `vue/vite.config.ts`, exactly
as `/desktop\.html$` already is — otherwise Workbox's navigation fallback serves the app shell
to an installed member following a public link, a silent and confusing failure.

**Watch the `generateSW` trap:** Workbox stringifies config functions into `sw.js`, so any
module-scope identifier used inside one becomes an undefined free variable at runtime. Use
literals.

## Acceptance Criteria

- [ ] `glimt` dataset in `config/offline.ts` with a budget and a sensible eviction position
- [ ] Separate `runtimeCaching` routes and budgets for thumbnails and full media
- [ ] Outbox excluded from eviction while it holds unsent media
- [ ] Public page path in `navigateFallbackDenylist`
- [ ] No module-scope identifiers inside Workbox config functions
- [ ] `npm run build` succeeds and the generated `sw.js` contains both routes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
