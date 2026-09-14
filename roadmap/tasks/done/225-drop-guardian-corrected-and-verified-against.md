# 225 — Remove `GuardianCorrected` and `verifiedAgainstPhone`

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

Task 148 introduced `Person.GuardianCorrected` and the `verifiedAgainstPhone` column so we
could tell "the member corrected our register" apart from "the register moved since the
member verified". Both read `PhoneParentRegistered` off the verification event, which task
222 removes.

PRD 015 §4 settles that neither question matters: the purpose is a check-in fast track, and
the only thing worth knowing is whether a verified contact number exists for this member.
So this is a deliberate removal, not collateral damage from the reshape.

Two consequences to state plainly rather than discover:

- **A verification now stands even if `phoneParent` later changes.** `IsVerified` stops
  invalidating on a register move. That is correct under the new framing — the member
  verified *a* reachable number and that is what check-in wanted — and would have been wrong
  under the old one.
- `verifiedAgainstPhone` becomes dead. Drop the column rather than leaving it as a NULL
  nobody reads, and remove `GuardianCorrected` rather than leaving a predicate returning a
  value it can no longer support.

Depends on task 222 (the field disappears there) and should land with or after task 224,
which touches the same projection and querier.

## Acceptance Criteria

- [x] `Person.GuardianCorrected` is gone, along with every caller and any API field derived
      from it
- [x] The `verifiedAgainstPhone` column is dropped from `table.sql` and from all reads and
      writes in `go/nathejk/table/person/`
- [x] `Person.IsVerified` no longer compares the verification against the current
      `phoneParent`, and a test asserts a verification survives a later `phoneParent` change
- [x] No test, fixture or annotation still refers to the removed field or column
- [x] The behaviour change (verification survives a register move) is recorded in a comment
      near `IsVerified`, naming PRD 015 as the decision

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — Picked up. Plan: `IsVerified` down to "verifiedAt and a number on file", delete
  `GuardianCorrected`, delete `invalidateVerification` and its call in
  `handleSpejderUpdated`, then drop the column from the struct, the SELECT lists, the schema
  and the drift list.
- 2026-09-12 — `IsVerified` is now `VerifiedAt != nil && PhoneParent != nil`. Kept the second
  half: with no number on file there is nothing for the tick to point at, which is a different
  claim from "the number changed". `GuardianCorrected`, `invalidateVerification` and the
  `VerifiedAgainstPhone` field/column/SELECT entries are gone. ✅ Criteria 1, 2, 3, 5.
- 2026-09-12 — Left `verifiedAgainstPhone` out of `table.go`'s `EnsureColumn` list rather than
  adding a DROP. That list is additive by design and documented as such; a destructive
  statement on every boot is exactly what the pattern exists to keep out. Fresh databases
  never get the column (it is out of `table.sql`); existing ones keep a NULL nobody reads
  until someone runs a real migration. Said so in a comment where the list ends.
- 2026-09-12 — Rewrote three projector tests that asserted the *old* rule (conditional clear,
  normalized comparison, clear-on-removal) into one that asserts the absence of a second
  statement. They were not wrong; they were right about a question PRD 015 dropped. The new
  test also fails if any statement still names the removed column.
- 2026-09-12 — Three `IsVerified` cases flipped from false to true, and each is marked FLIPPED
  in the table with the reason: register-moved-after, corrected-then-moved, and every
  verification recorded before `verifiedAgainstPhone` existed (which we used to refuse to
  vouch for — there is now nothing to be missing). The corresponding
  `confirmationRequired` case flipped too: a member whose register entry changed is no longer
  asked again.
- 2026-09-12 — Added `TestOwnPhoneVerificationIsNotAContactVerification`, which pins task
  224's trap from the read side. It passes structurally today (the querier does not select the
  own-phone columns), so its value is entirely in the future: whoever adds those fields to
  `Person` gets a failing test instead of an overlooked comment.
- 2026-09-12 — Replaced `TestVerifiedAtIgnoresSupersededAcknowledgement` with two tests: nil
  when no number is on file, and — the behaviour change itself — not nil after the register
  moves.
- 2026-09-12 — `gofmt -l`, `go vet ./...`, `go test ./...` and `GOWORK=off go build ./...` all
  clean. All criteria met.
