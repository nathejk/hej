# 003 — Mock user directory + phone recognition

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Implement phone-number recognition + role lookup in the BFF behind a small
**user-directory interface**, with a **mock** implementation (seeded, in-code
phone → role map) as the only implementation for now. The real Nathejk lookup
replaces it later without touching handlers. See
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`.

Roles to seed: `spejder`, `bandit` (main), `postmandskab`, `guide`, `samarit`.
Phone numbers must be normalized consistently (assume Danish `+45` default —
confirm) before lookup.

Depends on: 002.

## Acceptance Criteria

- [ ] A directory interface (e.g. `Lookup(phone) (User{id, role}, found bool)`)
      defined in `internal/` and consumed via a facade.
- [ ] A mock implementation seeded with at least one phone per role
      (spejder, bandit, postmandskab, guide, samarit).
- [ ] Phone normalization helper with unit tests (country code, spacing,
      leading-zero handling).
- [ ] Lookup is used only through the interface; swapping the implementation
      needs no handler changes.

## Progress Log

- 2026-07-30 13:12 — Task created.
