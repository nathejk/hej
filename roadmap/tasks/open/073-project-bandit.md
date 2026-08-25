# 073 — Project senior/klan as bandit

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §8. Consume `NATHEJK.*.senior.*.updated` / `.deleted`,
`NATHEJK.*.bandit.*.armNumber.assigned`, and the klan subjects.

"Bandit" is not a field anywhere: it is the event-role name for a senior in a
klan. The evidence is the subject vocabulary — shared-go's `senior` projector
consumes the `bandit.*.armNumber.assigned` subject and writes `senior.armNumber`.

Note `hq` **also** keeps bandits in its local `personnel` table, so bandit
identity is already split across two projections. Do not add a third notion of it;
derive from the senior/klan events only.

Seniors have **no guardian phone**.

## Acceptance Criteria

- [ ] Senior + klan + armNumber subjects consumed
- [ ] Classified as the `bandit` app role
- [ ] Arm number carried (it is an identification mechanism that needs no photo)
- [ ] Guardian phone explicitly "not applicable", not blank
- [ ] Idempotent, tested with `cqrstest` fakes

## Progress Log

- 2026-08-25 — Task created from PRD 006.
