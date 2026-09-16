# 291 — Load test /api/sync at expected device count

**Status:** open
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 §8: "Load, and it is the whole risk." This endpoint becomes the app's only
continuous during-race traffic besides position reporting, and it lands on the same BFF.
The mitigations are all in place by design — cached versions keyed on permitted set, a
tiny response, `ETag`, the debounce, the served interval — but *designed to be cheap* and
*measured cheap* are different claims, and the PRD says the measurement belongs in the
rollout rather than after it.

Six version derivations run per call. Each is a projection read or a 5 s cache hit, so
the interesting question is not the happy path but the **cache-miss storm**: a few
hundred devices foregrounding at once after a mass event (a race start, a broadcast
notification) all miss the same 5 s TTL together.

## What to measure

- Steady state: expected device count on a 60 s interval, everything unchanged.
- Thundering herd: all devices checking within one second, cold caches.
- Changed state: one dataset's version moves for every patrol at once (a mass reveal),
  so every device refetches a payload in the same window.
- Alongside position reporting at its own expected rate, since they share the BFF.

## Acceptance Criteria

- [ ] Steady-state p50/p95/p99 for `/api/sync` at expected device count, recorded.
- [ ] Thundering-herd p95 recorded; a stated verdict on whether the 5 s TTL needs
      single-flight protection.
- [ ] `/api/sync` p95 confirmed well inside the BFF's other read endpoints (PRD 017 §9).
- [ ] Cost of one `/api/sync` compared against one position report, as a ratio.
- [ ] Unchanged-response ratio in the steady-state run ≥ 95 %.
- [ ] Numbers written into PRD 017 §9, replacing the targets with measurements.
- [ ] A stated recommendation for the shipped interval, given the numbers.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Depends on task 284.
