# 071 — Phone-collision policy

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §11 Q1. Two people can share a phone number: siblings on one phone, or a
guardian's number entered as the scout's own. This is a **privacy** decision as
much as a UX one, and it decides whether `UNIQUE (year, phone)` can exist at all.

Options from the PRD:

1. **Refuse the login** — safe, but locks out a real participant
2. **Disambiguate after PIN verification** — best UX, more work; note the PIN
   proves control of the *number*, not identity, so a chooser is defensible
3. **Last-write-wins** — silently wrong, and exposes one person's data to whoever
   logs in

Option 3 is unacceptable. The interface shape matters more than the policy: if the
lookup returns a single user, the policy is baked in; if it returns all matches,
the caller can decide.

## Acceptance Criteria

- [ ] Decision recorded in PRD 006 §11 Q1 with reasoning
- [ ] Lookup API shaped so the policy is not silently baked in
- [ ] Collisions are logged loudly (they are a data problem to fix upstream)
- [ ] No path where one person sees another's data
- [ ] Schema constraint consistent with the decision
- [ ] Tests covering the collision case

## Progress Log

- 2026-08-25 — Task created from PRD 006.
