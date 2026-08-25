# 070 — GetByPhone / GetByID queriers and phone-normalization consistency

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §2/§6. The survey found **no phone lookup exists anywhere** across
shared-go, hq or tilmelding: no `ByPhone` query, no `Filter` accepting a phone, no
index on a phone column. This task adds the one the login path needs.

The correctness risk is normalization: a number stored by the projector and a
number typed at login must normalize **identically**, or lookups silently miss.
`go/internal/phone` already exists and is used by the login handler — the
projector must use the same function, not a second implementation.

## Acceptance Criteria

- [ ] `GetByPhone(ctx, year, phone)` and `GetByID(ctx, id)` on the person querier
- [ ] Index-backed lookup (index declared in the schema)
- [ ] Projector and login path share one normalization implementation
- [ ] A test proving a number stored via the projector is found by a login-shaped
      lookup, including a messy input form (spaces, +45, leading zeros)
- [ ] Not-found returns a clear zero value, not an error

## Progress Log

- 2026-08-25 — Task created from PRD 006.
