# 074 — Project crewmember and section

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §8. Consume `NATHEJK.*.crewmember.*.registered` / `.updated` / `.deleted`
/ `.section.assigned`, `NATHEJK.*.crew.*.signedup`, plus
`NATHEJK.*.section.*.added` / `.moved` / `.deleted` for the function labels.

Crew function comes from `sectionSlug` → the section tree, which is
organizer-authored. Use task 069's classification with its logged fallback.

Ordering matters and cannot be assumed: a `section.assigned` may arrive before the
`section.added` that names it. The projection must converge either way rather than
dropping the assignment.

## Acceptance Criteria

- [ ] Crewmember + crew + section subjects consumed
- [ ] Function derived through task 069's classifier
- [ ] Out-of-order section/assignment events converge
- [ ] Unassigned crew classified as generic crew, not an error
- [ ] `deleted` flag respected
- [ ] Idempotent, tested with `cqrstest` fakes incl. the out-of-order case

## Progress Log

- 2026-08-25 — Task created from PRD 006.
