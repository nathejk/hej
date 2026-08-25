# 069 — App-role classification and the section-slug map

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §8. Own the mapping from signup data to app role in one place, so no
handler or frontend infers a role from a team type or a slug.

Known mapping:

- **spejder** ← a `spejder` row (team is a `patrulje`)
- **bandit** ← a `senior` row (team is a `klan`); the giveaway is the subject
  `NATHEJK.*.bandit.*.armNumber.assigned` projecting onto `senior.armNumber`
- **gøgler** ← `NATHEJK.*.gøgler.*` events
- **crew** ← a `crewmember` row; the *function* comes from `sectionSlug`, which is
  organizer-authored free text

The crew part is the risky bit: `postmandskab`/`guide`/`samarit` rest on **string
convention validated by nothing**. Implement a slug→role map with a **logged
fallback to generic crew** — never fail a login over an unmapped slug.

## Acceptance Criteria

- [ ] A pure, unit-tested classification function (no DB, no stream)
- [ ] Slug→role map with normalization (case, whitespace, Danish characters)
- [ ] Unmapped slug → generic crew **and** a warning log, never an error
- [ ] Table-driven tests incl. unmapped, empty and mixed-case slugs
- [ ] Documented that this is convention, and what would replace it
      (projecting `section.Type`, PRD 006 §11)

## Progress Log

- 2026-08-25 — Task created from PRD 006.
