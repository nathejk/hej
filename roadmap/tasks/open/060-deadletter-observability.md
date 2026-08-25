# 060 — Deadletter observability

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §5/§6. The `cqrs.Writer` is `deadletter` wrapping `sqlpersister`. A
projection statement that cannot be applied must be **visible**, not swallowed —
a silently deadlettered event is a projection that is quietly wrong, which is the
worst failure mode in a CQRS system.

## Acceptance Criteria

- [ ] Deadlettered statements are logged with enough context to identify the
      subject and the failure
- [ ] A count//query is exposed (healthcheck field per task 059, a log metric, or
      both) so a non-zero deadletter count is discoverable without reading logs
- [ ] Documented where deadletters land and who is expected to watch them
      (PRD 008 §11 Q10 asks whether this belongs in shared infrastructure)
- [ ] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
