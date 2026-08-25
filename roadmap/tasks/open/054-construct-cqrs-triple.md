# 054 — Construct the cqrs triple in main.go

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §6/§8. Build the three interfaces every shared-go entity expects,
mirroring `hq`'s wiring:

| Interface | Role | Implementation |
|---|---|---|
| `cqrs.Reader` | query side | `*sql.DB` |
| `cqrs.Writer` | projection side | `deadletter` wrapping `sqlpersister` |
| `cqrs.Publisher` | command side | `metatagger` over JetStream |

Entity constructors take these interfaces — never a `*sql.DB` or a concrete
stream. Depends on tasks 050 (db) and 052 (deps).

## Acceptance Criteria

- [ ] `cqrs.Reader`, `cqrs.Writer`, `cqrs.Publisher` constructed in `main.go`
- [ ] JetStream connection established via `jrgensen/stream/jetstream`
- [ ] No handler or package outside `main.go` receives a `*sql.DB` directly
- [ ] Startup does not block on the broker (see task 058 for the full behaviour)
- [ ] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
