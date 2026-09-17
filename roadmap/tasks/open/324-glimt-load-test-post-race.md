# 324 — Load-test the post-race browse

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §0a.3, §9. **The load peak is the finish line, not the race.** A thousand people on the
same congested network at the same time, each paging through thousands of items, all pulling
media. This is the moment the feature is judged, and **it cannot be fixed on the night** — so it
gets measured before the first event.

Seed a realistic corpus (use `cmd/seed`): a full event's worth of holds, each with a plausible
number of glimt and media items. Then measure:

- `GET /api/glimt/hold/:number` — p50/p95/p99 and error rate under concurrent load
- `GET /api/glimt/feed` paging
- media and thumbnail serving, with and without the `immutable` cache hit

Record the numbers in this task file. PRD 019 §9 names p95 on the hold collection during the
hour after the race as a headline metric, so this task establishes the baseline it is measured
against.

## Acceptance Criteria

- [ ] A seed path producing a realistic corpus (documented item counts)
- [ ] Measured p50/p95/p99 and error rate for the hold collection, feed and media endpoints
- [ ] Numbers recorded in this task's progress log, with the hardware they were taken on
- [ ] Any index or query fix the measurements reveal is applied (or filed as a new task)
- [ ] Cache-hit vs cache-miss media cost quantified

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
