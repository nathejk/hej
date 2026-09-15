# 258 — Regression test: an un-revealed checkpoint never leaves the BFF

**Status:** open
**Priority:** high
**Created:** 2026-09-15

## Description

PRD 016 phase 2. The top risk of this whole feature (PRD 016 §8): widening the checkpoint
projection puts the positions of *un-revealed* posts one struct field away from a response.
The event area is deliberately not fully known to participants (PRD 002), and the race-area
hull exists precisely so that positions never have to leave.

The type-level boundary (task 255) is the primary defence. This task is the test that the
boundary holds, and it must be the kind of test that fails when someone adds a field in six
months rather than only when someone rewrites the query.

Scenario worth building: a year with checkpoints in three checkgroups, a patrol holding one
sheet covering two of them, and at least one positioned checkpoint that no rule reveals.
Then assert that its coordinates appear in **no** response body — `/api/checkpoints`,
`/api/patrol/handouts`, `/api/patrol/scans`, and `/api/race-area`'s polygon is a hull, not a
point list.

Also assert the guardian rule while we are here: no `phoneParent` in any of these payloads
(repo rule).

## Acceptance Criteria

- [ ] Test asserts an un-revealed checkpoint's latitude/longitude appear in no response.
- [ ] Test scans the serialised JSON, not just the Go structs, so a field added later is
      caught.
- [ ] Test covers all endpoints this PRD touches.
- [ ] Test asserts no `phoneParent` in any of them.
- [ ] A deliberately broken reveal rule makes the test fail (verified once by hand).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
