# 153 — Server-supplied grouping model for the contacts directory

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

Grouping comes from the server, never inferred client-side (PRD 007 §6, §8). Each
person in the manifest carries a **group id, a group label, and an own-group flag** so
the client renders sections without knowing what a klan or a section is.

Per population:

- bandit → grouped by **klan** (`users.User.PatrolID` / `PatrolName`), caller's own
  klan expanded by default;
- gøgler → one list including the crew gøglere;
- crew → one list.

**Why server-supplied rather than "group by klan" in the client:** *lok* — the grouping
of klaner that prompted the original PRD wording — is being migrated upstream into
**subsections**. Decision 2026-08-31 was to group by klan now, but the bandit view is
expected to gain a tier. If grouping is a server-supplied group, that is an additive
change to what the manifest emits; if the client hardcodes "klan", it is a rewrite.

So: do not assume one level of grouping is all there will ever be.

## Acceptance Criteria

- [ ] A group type carrying id, label and own-group flag, produced server-side.
- [ ] Bandit grouping derives from klan; gøgler and crew are single groups.
- [ ] A person with no klan lands in one clearly-labelled "Uden klan" group rather than
      disappearing.
- [ ] The shape does not preclude a second grouping tier (documented in a comment).
- [ ] Tests cover own-group flagging per role, and the no-klan fallback.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8.
