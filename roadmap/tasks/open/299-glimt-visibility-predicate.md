# 299 — Glimt visibility predicate in internal/users

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §8. One definition of "may this caller see this glimt", so the feed query and the
media handler cannot disagree. Lives in `go/internal/users/` beside `MayList` /
`MayLookUpPatrol`.

Two pieces:

1. **Role → group mapping.** `spejder` → `spejder`; `bandit` → `bandit`; all of
   `postmandskab`, `guide`, `samarit`, `gøgler`, `crew` → `crew`. Mirrors `IsCrewRole`.
2. **May-see rule.** Given the caller's group and moderator flag, and a glimt's audience +
   author group: `public` → everyone; `nathejk` → any authenticated member; `group` → same
   group only. Plus a **Team-section override** that sees every scope.

The override must live **inside** the predicate, not as a branch around it at each call site
— if a handler can reach media by skipping the check, the check is no longer the only thing
between a group-scoped photo and the world (PRD 019 §8).

Hidden glimt (`hiddenAt` set) are visible to the author and to moderators, and to nobody
else. That belongs in this predicate too.

## Acceptance Criteria

- [ ] `users.GlimtGroup(role)` maps all seven roles, with a test naming each
- [ ] `users.MaySeeGlimt(...)` covers public / nathejk / group / hidden, with the moderator override inside it
- [ ] Table-driven test asserts a spejder cannot see a crew `group` glimt, and vice versa
- [ ] Test asserts a moderator sees a `group` glimt from a group they are not in
- [ ] Test asserts a hidden glimt is invisible to a normal member of the right group
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
