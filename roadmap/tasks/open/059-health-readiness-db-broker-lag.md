# 059 — Health/readiness reporting DB, broker and projection lag

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §6. Extend the existing `GET /api/healthcheck` so it reports database
connectivity, broker connectivity and projection lag.

Critically: **readiness must not fail on broker absence** (task 058 — the app is
deliberately useful without it), but it must still be *visible* that the broker
is down, or a silent outage becomes stale data nobody notices.

Endpoint changes need OpenAPI annotations per `.rules`.

## Acceptance Criteria

- [ ] Healthcheck reports database reachability
- [ ] Healthcheck reports broker connectivity as informational, not fatal
- [ ] Healthcheck reports projection lag (or explicitly "unknown" while no
      projections exist)
- [ ] Readiness stays green with the broker down and the database up
- [ ] Readiness goes red with the database down
- [ ] OpenAPI annotations updated
- [ ] Tests cover the green/red matrix

## Progress Log

- 2026-08-25 — Task created from PRD 008.
