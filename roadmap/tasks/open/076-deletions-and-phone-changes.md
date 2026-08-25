# 076 — Handle deletions and phone changes

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §5. Two mutations that are easy to miss and both have security weight:

- **Deletion** (`spejder.deleted`, `senior.deleted`, `crewmember.deleted`): a
  deleted member must lose their login. A projection that only ever inserts leaves
  a working credential behind.
- **Phone change**: the old number must stop resolving, or two numbers log in as
  one person and a reassigned number logs in as the wrong one.

A phone change should also invalidate PRD 005's verification if the *guardian*
number changed — the acknowledged number is stored precisely so that is decidable.

## Acceptance Criteria

- [ ] Delete events remove the person from the directory
- [ ] A changed phone stops resolving at the old value
- [ ] Guardian-number change clears `verifiedAt` (or is documented as not doing so)
- [ ] Tests for delete, phone change and re-add
- [ ] Idempotent under replay

## Progress Log

- 2026-08-25 — Task created from PRD 006.
