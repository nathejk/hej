# 133 — BFF: publish the verification event

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] The verification is published through `commands.Commands`, using the message from
      task 132
- [ ] No direct SQL write anywhere on the confirm path
- [ ] The publish lives in `go/cmd/api/profile.go`, not a new parallel handler file
- [ ] The published payload includes the acknowledged number and the timestamp, so the
      projection in task 134 has everything it needs from the log alone
- [ ] A publish failure surfaces as a server error rather than a silent success — a
      confirmation the log never saw must not report `204`
- [ ] `hej` calls no other service's API as part of this

## Depends on

- **Task 132** — the message must exist and be released in shared-go first.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
