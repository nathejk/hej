# 065 — Local data seeding / replay procedure for developers

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §11 Q7. A projection cannot be verified against an empty table, so
developers need realistic data. Options: replay from a real stream, an anonymised
dump, or seeded fixtures.

**Privacy constraint:** the data is minors' names, addresses and guardian phone
numbers. An anonymised or synthetic path is very likely the only acceptable one —
do not document a procedure that copies production personal data onto laptops.

## Acceptance Criteria

- [ ] A documented, repeatable way to get usable local data
- [ ] No procedure that puts real personal data on a developer machine
- [ ] Works from an empty volume (proves replay/rebuild too)
- [ ] Referenced from the dev docs so it is discoverable

## Progress Log

- 2026-08-25 — Task created from PRD 008.
