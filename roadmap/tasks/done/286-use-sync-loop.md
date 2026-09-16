# 286 — useSyncLoop: one app-level loop dispatching per-dataset refreshes

**Status:** done
**Priority:** high
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

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

- [x] `useSyncLoop.ts` fetching `/api/sync` through `fetchWrapper`.
- [x] Dispatches per-dataset refreshes for differing versions only.
- [x] Registered once in `App.vue`.
- [x] Absent key → no request for that dataset.
- [x] `unavailable` key → cached copy kept, no refetch, still asked next check.
- [x] One dataset's failure does not block the others or stop the loop.
- [x] `401` stops the loop.
- [x] Exposes a forced `check` for task 282's manual refresh.
- [x] Tests with an injected `FreshnessTarget` and a stubbed fetch: dispatch-on-differ,
      no-dispatch-on-same, absent, unavailable, one-fails-others-apply, 401 stops.
- [x] `npm run type-check` and the unit suite green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1.
- 2026-09-16 14:00 — Picked up. Plan: `useSyncLoop` wraps `useFreshnessLoop`, fetches
  `/api/sync`, and dispatches through a table keyed by the `SyncDataset` union from task
  287 — so a dataset the server reports and the client forgets to handle is a type error
  rather than a dataset that silently never refreshes. Registered in `App.vue`; task 288
  removes the two loops it replaces.
- 2026-09-16 14:20 — **The dispatch table is a `Record` over the `SyncDataset` union**, not a
  string lookup. A dataset the server can report and the client forgot to handle is now a
  type error rather than a dataset that silently never refreshes — which would look exactly
  like the feature working. The inverse case (a server ahead of an installed client, which is
  normal for a PWA) is handled the other way: an unknown *name* is ignored so the datasets
  this build knows keep working.
- 2026-09-16 14:25 — **Needed runtime setters on `useFreshnessLoop`**
  (`setIntervalSeconds` / `setDebounceSeconds`), so this landed here rather than in task
  289: the served values change *while the loop runs*, and the loop could not be wired at all
  without a way to adopt them. Interval changes restart the timer rather than leaving it on
  the old period — the whole point of the 02:00 lever on a device that may not be reloaded
  for hours. 289 keeps the verification.
- 2026-09-16 14:30 — `race_area` is accepted and ignored: it has no client store (see task
  287's log and task 294). Written as an explicit table entry with a comment rather than
  left out, so it reads as a decision rather than an omission — and so the exhaustive
  `Record` type keeps working.
- 2026-09-16 14:35 — A 401 sets a latch that disables the loop through `enabled`. Retrying
  an expired session on every foreground for the rest of the app's life is the failure this
  avoids; the existing auth handling owns the redirect, so this only stops the traffic.
  `reportDirectory()` is called **only after an actual refresh**, or the readiness screen
  would claim a sync happened on every check that fetched nothing.
- 2026-09-16 14:40 — Completed. `useSyncLoop.ts` + `useSyncLoop.spec.ts` (12 tests),
  registered in `App.vue` alongside the loops task 288 will remove. `npm run type-check`
  clean; full suite 703 passed (57 files).
