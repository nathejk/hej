# 067 — Extend the app-role enum with gøgler and generic crew

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §6. The app-role enum is `spejder | bandit | postmandskab | guide |
samarit` in both `go/internal/users/directory.go` and
`vue/src/stores/session.store.ts`. PRD 006 must cover **gøgler** and a generic
**crew** fallback, and neither value exists — a gøgler login is currently
untypeable, and an unclassifiable crew member has no role to be given.

Both new values need a navigation answer too (`vue/src/config/navigation.ts`).
Generic `crew` must be **least-privileged**: it exists because classification
failed, so it must not inherit what identified crew functions get (PRD 007's
access matrix depends on this).

## Acceptance Criteria

- [ ] `users.Role` gains `gøgler` and a generic crew value
- [ ] `session.store.ts` `Role` type matches exactly
- [ ] `navigation.ts` states what each new role sees
- [ ] Generic crew is documented as least-privileged, not "unrestricted"
- [ ] Existing role-gated routes still behave for the five original roles
- [ ] Go and frontend builds/tests green

## Progress Log

- 2026-08-25 — Task created from PRD 006.
