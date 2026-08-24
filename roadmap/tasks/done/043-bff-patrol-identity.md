# 043 — BFF: patrol identity on the user directory

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5, delegated sub-agent)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

PRD 002 needs to resolve a signed-in user's patrol; `internal/users.User` carried
only `{ID, Role}`. Extend the directory behind its existing interface so the real
Nathejk lookup can still replace the mock without touching handlers. PRD 003 will
need name/address/phone on the same directory, so the shape should accommodate that
without an interface change.

## Acceptance Criteria

- [x] `users.User` carries a patrol identity (`PatrolID`, `PatrolName`).
- [x] `Directory` can resolve a user from a **session** (which carries only a user
      id), not just from a phone number.
- [x] Mock seeds patrol data for the spejder and bandit roles; personnel roles have
      no patrol.
- [x] Existing tests still pass; new behaviour covered.
- [x] `session.Session` unchanged.

## Progress Log

- 2026-08-24 12:10 — Delegated to a sub-agent with a write scope limited to `go/`,
  running in parallel with the frontend work (disjoint files).
- 2026-08-24 12:35 — Done. `users.User` gained `PatrolID` + `PatrolName` (empty
  `PatrolID` = "no patrol", the normal state for personnel), and `Directory` gained
  `Get(id string)` so a session resolves without the phone number it was created
  from. Seeded ids are exported as `users.MockSpejderPatrolID` /
  `MockBanditPatrolID` and the scan fixture keys off them, so the two mocks cannot
  drift apart.
- 2026-08-24 12:36 — Good call by the sub-agent: it left `session.Session` alone
  deliberately, because baking the patrol into the signed cookie would freeze it
  for the session's 7-day lifetime.
- 2026-08-24 12:37 — PRD 003's fields are additive on the same struct with no
  interface change, as required. They were **not** implemented here.
