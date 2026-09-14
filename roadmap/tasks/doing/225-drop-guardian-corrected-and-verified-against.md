# 225 — Remove `GuardianCorrected` and `verifiedAgainstPhone`

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `Person.GuardianCorrected` is gone, along with every caller and any API field derived
      from it
- [ ] The `verifiedAgainstPhone` column is dropped from `table.sql` and from all reads and
      writes in `go/nathejk/table/person/`
- [ ] `Person.IsVerified` no longer compares the verification against the current
      `phoneParent`, and a test asserts a verification survives a later `phoneParent` change
- [ ] No test, fixture or annotation still refers to the removed field or column
- [ ] The behaviour change (verification survives a register move) is recorded in a comment
      near `IsVerified`, naming PRD 015 as the decision

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
