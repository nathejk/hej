# 056 — Make commands.Commands a real publisher-backed write facade

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §8. `internal/commands.Commands` is an empty struct. Under the
architecture rule that **nothing writes directly to the database**, this is the
only write path handlers may use: every state change publishes an event, and SQL
is projection-only.

Give it the publisher, a way to publish an event, and an interface shape that
handlers can depend on. No domain commands exist yet (PRD 005's verification
event is the first consumer), so this task lands the seam and its test double —
not a specific command.

Note `go-bff-layout` says write-side APIs are not a package in the mature repos:
`cmd/api/main.go` collects each entity's `Commands` interface. `hej` still has
`internal/commands`; keep it for now but shape it so entity-owned command
interfaces can replace it without touching handlers.

## Acceptance Criteria

- [ ] `Commands` carries a `cqrs.Publisher` and exposes a publish path
- [ ] Handlers can be given the facade without importing `cqrs` or `stream`
- [ ] A test double exists (use `cqrs/cqrstest` fakes) so command paths are
      testable with no broker
- [ ] A comment records that direct SQL writes are forbidden and why
- [ ] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
