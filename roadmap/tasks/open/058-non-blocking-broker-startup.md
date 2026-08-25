# 058 — Non-blocking broker startup and degraded-mode reads

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §5/§6. During an event, degraded and serving beats correct and dead. A
broker outage must not take the app down: reads come from SQL projections, so a
stale-but-present projection is far better than an unbootable API.

Startup must therefore not block on JetStream, and a broker that disappears at
runtime must not crash the process or wedge request handling. Reconnection should
be attempted in the background.

Note the deliberate asymmetry: the **database** IS on the login path, so a
database outage does break login (PRD 008 §5 accepts this). The broker is not.

## Acceptance Criteria

- [ ] API starts and serves reads with the broker unreachable
- [ ] Broker connection attempted asynchronously with retry/backoff
- [ ] Loss of the broker at runtime is logged, not fatal
- [ ] Publish attempts with no broker fail the request cleanly (a clear error,
      no panic, no silent success)
- [ ] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
