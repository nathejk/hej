# 300 — mayModerate: per-request Team-section lookup

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §8. Moderation authority belongs to the **Team section** (slug `team`) — the
organizers and the people responsible.

**It cannot come from the session.** `session.Session` carries only `UserID`, `Role` and
`ExpiresAt`, and it is a stateless signed cookie with no server-side store, so a
`section: "team"` claim would keep working after the assignment was revoked for as long as
the cookie lived, with no way to invalidate it. Instead: look up the caller's current
`sectionSlug` from the `person` table (the column exists) on every moderation request. One
indexed read, correct revocation.

The slug must be a named constant — `SectionTeam = "team"` — declared next to
`crewFunctionBySlug` in `go/nathejk/table/person/classify.go`. Section slugs are
organizer-authored and validated by nothing, and `"team"` is an easy literal to typo or to
confuse with the `team*` columns (the hold's number and name) a few lines away. See PRD 019
§0b on the naming collision.

Fail closed: no DB, no person row, or a different slug → not a moderator.

## Acceptance Criteria

- [ ] `person.SectionTeam` constant declared next to `crewFunctionBySlug`
- [ ] A querier method returns the current `sectionSlug` for a `personId` + year
- [ ] `mayModerate` in `go/internal/users/` (or a thin app method delegating to it) is the single definition
- [ ] Test: person in section `team` → moderator
- [ ] Test: person in any other section → not a moderator
- [ ] Test: **revocation** — same person, section changed, no longer a moderator on the next call
- [ ] Test: missing person row / no DB → not a moderator (fails closed)
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
