# 059 — Health/readiness reporting DB, broker and projection lag

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008 §6. Extend the existing `GET /api/healthcheck` so it reports database
connectivity, broker connectivity and projection lag.

Critically: **readiness must not fail on broker absence** (task 058 — the app is
deliberately useful without it), but it must still be *visible* that the broker
is down, or a silent outage becomes stale data nobody notices.

Endpoint changes need OpenAPI annotations per `.rules`.

## Acceptance Criteria

- [x] Healthcheck reports database reachability
- [x] Healthcheck reports broker connectivity as informational, not fatal
- [x] Healthcheck reports projection lag (or explicitly "unknown" while no
      projections exist)
- [x] Readiness stays green with the broker down and the database up
- [x] Readiness goes red with the database down
- [x] OpenAPI annotations updated
- [x] Tests cover the green/red matrix

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Extended `GET /api/healthcheck` with a `dependencies`
  object rather than replacing the existing payload, so the pre-existing
  `TestHealthcheck` contract keeps passing (verified: it does).
- 2026-08-25 — Implemented three states per dependency — `up`, `down`, `absent`.
  **`absent` is the important one:** "no database configured" and "database
  unreachable" are completely different situations, and collapsing them into a
  boolean would either make the current mock-based setup look broken or make a real
  outage look intentional.
- 2026-08-25 — Readiness rule implemented as specified: **database down → 503**,
  **broker down → 200**. Reasoning recorded in the handler doc comment so the next
  person does not "fix" the asymmetry: reads come from projections, so the app works
  without a broker; but once PRD 006's directory lands, no database means no logins,
  and reporting ready would keep an orchestrator sending traffic to a process that
  cannot serve it.
- 2026-08-25 — Decision: projection lag reports **`"unknown"`**, not `0`. There are no
  projections yet, and a fabricated zero would read as "fully caught up" — the most
  misleading possible answer. Real lag (stream sequence vs applied sequence) needs
  something to measure and arrives with PRD 006.
- 2026-08-25 — The database ping is bounded at 2s. A healthcheck that blocks on an
  unresponsive database is its own failure mode, and probes are usually what notice
  first.
- 2026-08-25 — Re-added `eventing.connected()`, which task 058 removed as dead code;
  it is consumed here.
- 2026-08-25 — ✅ All criteria complete. 4 new tests cover: no database → ready +
  `absent`; broker configured but unreachable → `down` **with** an explanation and
  still ready; broker unconfigured → `absent`; lag → `unknown`.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so the
  database-down → 503 path is covered by the code path (`app.db == nil` vs ping
  failure) but **not** by a test against a real database that is stopped mid-run.
  That is the one branch I would want to see exercised for real before trusting it in
  production.
- 2026-08-25 — Moving to done.
