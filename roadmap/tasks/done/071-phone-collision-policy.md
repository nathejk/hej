# 071 — Phone-collision policy

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

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

- [x] Decision recorded in PRD 006 §11 Q1 with reasoning
- [x] Lookup API shaped so the policy is not silently baked in
- [x] Collisions are logged loudly (they are a data problem to fix upstream)
- [x] No path where one person sees another's data
- [x] Schema constraint consistent with the decision
- [x] Tests covering the collision case

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-25 — **Decision from the maintainer: disambiguate after PIN verification.**
  Recorded in PRD 006 §11 Q1 with the reasoning — the PIN proves control of the
  *number*, not which of its owners is holding it, so verify first and then ask.
- 2026-08-25 — Put the policy in the **directory contract** rather than in a handler,
  because a policy living in one call site is a policy the next call site will get
  wrong:
  - `LookupAll(phone) []User` — every owner
  - `Lookup(phone) (User, bool)` — the single owner, and **not found** when shared
  That asymmetry is the safety property: a caller who has not thought about
  collisions gets a refused login (visible, fixable) instead of silently
  authenticating someone as their sibling.
- 2026-08-25 — Auditing the two existing call sites showed they need *opposite*
  treatment, which is exactly why both methods exist:
  - `request-pin` now uses **`LookupAll`**. A shared number is still a recognized
    number and must still receive an SMS — using `Lookup` there would have silently
    refused to text precisely the users who need the new flow.
  - `verify` uses `LookupAll` to detect the collision, logs a warning, and refuses
    until the chooser exists.
- 2026-08-25 — Seeded a **shared number with two siblings** in the mock directory
  (`MockSharedPhone`). Without a fixture the collision path would first be exercised
  in production, on real children's data.
- 2026-08-25 — Schema is already consistent: task 068 made `year_phone` a plain KEY
  rather than UNIQUE, so the projector never fails on this data — which is what lets
  the policy be a decision at login time instead of a constraint violation at
  projection time.
- 2026-08-25 — Created **task 079** for the chooser flow itself (post-verification
  signed token, candidate endpoint, login-step UI). It belongs to the auth/PRD 005
  surface, not to the directory. Flagged there and here: **until 079 ships a shared
  number cannot log in at all**, so 079 must land before or with task 077's swap to
  the real projection, or real colliding users are locked out of production. Chose a
  fail-safe over a fail-open deliberately.
- 2026-08-25 — Noted the privacy shape for 079 rather than leaving it to be
  rediscovered: the chooser necessarily shows one person the names of the others on
  that number, so the payload must be a first name and maybe a team — not addresses,
  not full profiles — and the endpoint must be unreachable without a verified PIN.
- 2026-08-25 — ✅ All criteria complete. 6 new tests: all owners returned, shared
  number refused by `Lookup`, unique number still resolves, **stable ordering** (a
  reshuffling chooser is a confusing chooser), unknown number empty, and both owners
  resolvable by id (needed to restore a session once one is chosen).
- 2026-08-25 — Verification caveat: no real collision data exists to test against —
  the fixture is invented. How common this is in the real signup data is still
  unmeasured; task 078 counts it.
- 2026-08-25 — Moving to done.
