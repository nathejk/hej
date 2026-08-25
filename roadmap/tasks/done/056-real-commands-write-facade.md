# 056 — Make commands.Commands a real publisher-backed write facade

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

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

- [x] `Commands` carries a `cqrs.Publisher` and exposes a publish path
- [x] Handlers can be given the facade without importing `cqrs` or `stream`
- [x] A test double exists (use `cqrs/cqrstest` fakes) so command paths are
      testable with no broker
- [x] A comment records that direct SQL writes are forbidden and why
- [x] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Kept `internal/commands` for now (the mature siblings
  collect entity command interfaces in `cmd/api` instead) but shaped it as a
  collection point so that migration is later mechanical.
- 2026-08-25 — Decision: **a write with no broker fails the request.** This is the
  deliberate asymmetry with the read path — reads survive a broker outage by serving
  existing projections (PRD 008 §5), but an event that could not be published has
  not happened, and reporting success would tell a member their guardian number was
  confirmed when nothing recorded it (PRD 005). Exposed as `ErrNoPublisher` plus an
  `Available()` check so a handler can fail fast with a clear message.
- 2026-08-25 — Added `publisherFor(ev)` in `cmd/api` rather than nil-checking inline.
  The reason is specific: assigning a typed-nil `*metatagger.Publisher` into the
  `cqrs.Publisher` interface would make `Available()` report **true** and then panic
  on first use. A helper that returns an untyped nil avoids that trap.
- 2026-08-25 — `SetBody` returns an error and I was discarding it. Fixed: an
  unmarshallable body now fails instead of publishing an event with an empty body,
  because a subscriber cannot tell that apart from one with no fields set.
- 2026-08-25 — Learned something worth recording: `cqrs.SubjectFromStr` accepts `:`
  or `.` between domain and type but **normalises to `.`**, so a round-tripped
  subject does not equal its input string. My first test asserted on the input form
  and failed. Both forms are now named constants in the test with a comment, since
  the difference is invisible until you assert on it.
- 2026-08-25 — Also fixed: `stream.Subject` has no `String()` method — the accessor
  is `Subject()`. And `routes_test.go` needed updating for the new `New(publisher)`
  signature; it passes nil, which is now a supported mode.
- 2026-08-25 — ✅ All criteria complete. 4 tests via `cqrstest.Publisher`: no-publisher
  refusal, one-message-on-subject, publisher-error propagation, unmarshallable body.
  Build/test/vet/staticcheck/gofmt green on both the workspace and `GOWORK=off`
  paths.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**; no event was
  published to a real broker. Covered only by the in-memory fake.
- 2026-08-25 — Moving to done.
