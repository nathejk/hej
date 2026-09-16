# 286 — useSyncLoop: one app-level loop dispatching per-dataset refreshes

**Status:** open
**Priority:** high
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 phase 1. New `vue/src/composables/useSyncLoop.ts`: a single app-level loop
whose `check` fetches `/api/sync` and dispatches a refresh to each store whose version
differs from what it holds. Registered **once** in `App.vue`, beside the existing
app-level concerns.

Built on `useFreshnessLoop` (with task 281's debounce), so the trigger-point decisions
stay in one place and stay testable without a DOM.

Rules it must honour (PRD 017 §6):

- **Only datasets present in the response are consulted.** An absent key means the user
  may not hold it — do not request it, do not warn, do not fall back to a client-side
  role table.
- **`unavailable` is not absence.** Keep the cached copy, do not refetch, keep asking on
  the next check.
- **Per-dataset failure is isolated.** One store's refresh rejecting must not prevent
  the other five from applying, and must not abort the loop. Record staleness (PRD 009).
- **Refetches are independent and may apply out of order.** Nothing may assume
  checkpoints and handouts are mutually consistent within one check.
- **Metadata before images**: a corrected phone number may arrive before the new
  portrait, never the reverse.
- **A `401` stops the loop** rather than retrying every foreground; the existing auth
  handling owns the redirect.

Depends on task 287 for the uniform `refreshIfVersionDiffers` on each store.

## Acceptance Criteria

- [ ] `useSyncLoop.ts` fetching `/api/sync` through `fetchWrapper`.
- [ ] Dispatches per-dataset refreshes for differing versions only.
- [ ] Registered once in `App.vue`.
- [ ] Absent key → no request for that dataset.
- [ ] `unavailable` key → cached copy kept, no refetch, still asked next check.
- [ ] One dataset's failure does not block the others or stop the loop.
- [ ] `401` stops the loop.
- [ ] Exposes a forced `check` for task 282's manual refresh.
- [ ] Tests with an injected `FreshnessTarget` and a stubbed fetch: dispatch-on-differ,
      no-dispatch-on-same, absent, unavailable, one-fails-others-apply, 401 stops.
- [ ] `npm run type-check` and the unit suite green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1.
