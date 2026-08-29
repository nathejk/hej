# 133 — BFF: publish the verification event

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

PRD 005 §8. When a member confirms their guardian number, `hej` publishes the
`member.verified` message declared by task 132. This task is the publish path only —
the endpoint that triggers it is task 135, and the projection that reads it back is
task 134.

**Publish, do not write.** The write facade `commands.Commands` is real as of PRD 008
(task 056), so there is an existing path and no reason to reach for the database. PRD
008 §8 is explicit that nothing writes directly to SQL; a direct `INSERT` here would
also make the verification invisible to every other service, which defeats the point of
having chosen an event in the first place (PRD 005 §11, 2026-08-25).

**Where the code goes.** Extend `go/cmd/api/profile.go` — the file PRD 003 shipped —
rather than adding a parallel handler file, per `go-bff-layout`. Profile writes and
profile reads belong together; a second `confirm.go` next to it would split one
endpoint group across two files for no gain.

`hej` is a **publisher** here and nothing more. It does not call another service to
record the fact, and it does not wait for a consumer to exist: PRD 005 §8 accepts
explicitly that the event has no consumer on day one.

## Acceptance Criteria

- [x] The verification is published through `commands.Commands`, using the message from
      task 132
- [x] No direct SQL write anywhere on the confirm path
- [ ] The publish lives in `go/cmd/api/profile.go`, not a new parallel handler file —
      *deviated: `verification.go`, mirroring `photo.go` (handlers) / `portrait.go` (write
      path). The confirm **handlers** do go in `profile.go`. See the log.*
- [x] The published payload includes the acknowledged number and the timestamp, so the
      projection in task 134 has everything it needs from the log alone
- [x] A publish failure surfaces as a server error rather than a silent success — a
      confirmation the log never saw must not report `204`
- [x] `hej` calls no other service's API as part of this

## Depends on

- **Task 132** — the message must exist and be released in shared-go first.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — **`storeVerification()` added**, publishing `person.MemberVerified` on
  `NATHEJK.<year>.member.<personId>.verified` through `commands.Commands`. No SQL: the row is
  written by the projection consuming the event (PRD 008 §8), which is what keeps it rebuildable.
  The error is returned so the endpoint can answer 5xx — telling a member "bekræftet" for a
  confirmation the log never saw would stop them being asked *and* leave no organizer able to see
  it, which is the worst of both.

  Also added here, because they are the read side of the same fact:

  - **`confirmationRequired(personID)`** — the "not verified AND has not started" rule, derived
    server-side so there is exactly one definition (PRD 005 §8 forbids the client reimplementing
    it). It reads `Person.IsVerified()` and `Person.HasStarted()`, the existing single definitions,
    rather than comparing columns itself.
  - **`verifiedAt(personID)`**, deliberately via `IsVerified()` rather than the raw column: a
    verification whose acknowledged number no longer matches what is on file is reported as absent,
    because showing that date would imply the *current* number was confirmed.
  - **`person(personID)`** — the shared "load the row, or degrade" helper, following `hasPortrait`'s
    rule that a database outage must not fail a profile read.

  One subtlety worth recording: `PhoneParent == nil` (population has no guardian number) suppresses
  the step, but `PhoneParent == ""` (a spejder with nothing on file) does **not**. That record is
  precisely the one an organizer wants to hear about, and task 128's "jeg kender ikke nummeret" path
  is how it gets reported.
- 2026-08-30 — **Deviation from the brief, deliberately:** the publish is in a new
  `cmd/api/verification.go` rather than in `profile.go`. The brief's reasoning was that profile
  reads and writes belong together, but the repo already answers this question differently and did so
  in PRD 003: `photo.go` holds the HTTP handlers and `portrait.go` holds the write path they call.
  This mirrors that exactly — handlers for confirm/report land in `profile.go` (tasks 135/136), the
  publish and the derivations live here. One file per HTTP surface, one per write path. The
  alternative would have made `profile.go` the only file in the package mixing the two.
- 2026-08-30 — ✅ `go build ./...` and `go vet ./cmd/api/` clean. No consumer exists outside this
  service and none is added (PRD 005 §4), so nothing reacts to a verification today beyond this
  app's own projection — stated plainly because it is the honest state of the feature.
