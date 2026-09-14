# 224 — Project own-phone and contact verification into separate columns

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

The reshaped message (task 222) carries two independent facts, and the projection in
`go/nathejk/table/person/` currently has one column for them. Merging them would invert the
whole point of PRD 015.

Today `verifiedAt` means "the contact number was confirmed", and `Person.IsVerified` gates
`confirmation_required` on it (`querier.go`). A skip or exhaustion event carries a
`VerifiedAt` too — it is a real verification, of the member's *own* number, proven by the
PIN. So writing that timestamp into the existing column would mark **every member who gave
up** as verified, silence the question on their next login, and fast-track at check-in
exactly the members whose records still need a contact number. That is the opposite of what
this PRD exists to do.

So: add a separate own-phone column (`phoneVerifiedAt` or similar) and leave the contact
column meaning only what it means today. `IsVerified` and `confirmationRequired` keep
reading the contact column and nothing else.

Second change in the same handler: `handleMemberVerified` currently **returns an error**
when there is no acknowledged phone number. Under the new shape that would dead-letter every
login event and every skip event, because neither carries a `PhoneContact`. The rule it was
protecting — never record a contact verification that names no number — survives as: an
event without `PhoneContact` writes **only** the own-phone column.

Comment the sharp edge at the consume end (PRD 015 §6): an absent `PhoneContact` from a
login says nothing, while the guardian-check endpoints are the only publishers that may
assert the field. A login must never be able to un-verify a contact number.

Depends on tasks 222 and 223.

## Acceptance Criteria

- [x] Own-phone verification is stored in its own column; the existing contact-verification
      column keeps its current meaning
- [x] `Person.IsVerified` and `confirmation_required` are unchanged for a member who has
      only an own-phone verification — a skip does not silence the check
- [x] `handleMemberVerified` no longer errors on an event with no `PhoneContact`; it writes
      only the own-phone column and returns success
- [x] An event carrying both numbers writes both columns
- [x] A login-shaped event (no `PhoneContact`) cannot clear an existing contact verification
- [x] Consumer tests cover all three event shapes: own-phone only, both numbers, and a
      guardian-check event with an empty `PhoneContact`

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — Picked up. Plan: `phoneVerifiedAt` + `verifiedPhone` columns via the existing
  additive-drift list in `table.go`, then split `handleMemberVerified` into "write what this
  event establishes" so an absent field writes nothing rather than clearing anything.
- 2026-09-12 — Columns added in `table.sql` (fresh databases) and in `table.go`'s
  `EnsureColumn` list (existing ones), following the documented additive-drift pattern.
  Rewrote the schema comment on the whole verification block: it still described the
  three-column staleness scheme, which task 225 is removing.
- 2026-09-12 — `handleMemberVerified` now builds its SET list from the fields that are
  *present*, rather than writing a fixed set of columns. That is the shape that makes "absent
  says nothing" true by construction instead of by care: there is no code path in which a
  login event names the contact columns at all. Refuses only when both numbers are absent.
- 2026-09-12 — ✅ Criteria 1, 3, 4, 5. Four new consumer tests: own-phone-only (asserting the
  *absences* — no `acknowledgedPhone`, no `verifiedAt`, no `verifiedAgainstPhone`), both
  numbers, both-empty refusal, and one asserting a skip event and a login event project
  *identically*, which is the `omitempty` decision from PRD 015 §6 pinned down as a test
  rather than left as a comment.
- 2026-09-12 — Criterion 2 holds structurally rather than by assertion, which is better:
  `IsVerified` reads `VerifiedAt`/`AcknowledgedPhone`/`VerifiedAgainstPhone` off the row, and
  the querier does not select the new columns at all (no `SELECT *` anywhere in the package,
  checked), so an own-phone verification is not merely ignored by the check — it is invisible
  to it. Noted here because a future reader may wonder why there is no test for it.
- 2026-09-12 — `go test ./...` clean with the workspace active.
- 2026-09-12 — **Could not verify the migration against the running dev database.** The api
  container's dev loop has a `pinned_build` gate (`docker/init/api-dev`) that compiles with
  `GOWORK=off` exactly to catch code depending on symbols that exist only in the sibling
  shared-go checkout — which is precisely where PRD 015 is right now. It correctly refuses to
  start, printing "push shared-go, then bump: go get github.com/nathejk/shared-go@latest". So
  the `EnsureColumn` calls are unexecuted so far. Confirmed by hand that `go build ./...`
  succeeds *inside* the container (the mount and go.work are wired correctly), so the only
  thing standing between this and a running api is the version bump from task 222.
