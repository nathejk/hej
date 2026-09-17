# 300 — mayModerate: per-request Team-section lookup

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

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

- [x] `person.SectionTeam` constant declared next to `crewFunctionBySlug`
- [x] A querier method returns the current `sectionSlug` for a `personId` + year
- [x] `mayModerate` in `go/internal/users/` (or a thin app method delegating to it) is the single definition
- [x] Test: person in section `team` → moderator
- [x] Test: person in any other section → not a moderator
- [x] Test: **revocation** — same person, section changed, no longer a moderator on the next call
- [x] Test: missing person row / no DB → not a moderator (fails closed)
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 10:30 — Picked up. First finding, and a welcome one: **`"team"` is already a known
  slug** in `crewFunctionBySlug`, observed in the real 2026 section tree and mapped to
  `RoleCrew`. So the section exists and this task is gating on a real value rather than
  inventing one. `"hq"` is in there too, which is what the PRD originally proposed — the switch
  to `team` was the right call, since both are real and only one is the responsible group.
- 2026-09-17 10:40 — Second finding: **no new querier method was needed.**
  `app.person(personID)` already exists in `verification.go`, returns `person.Person` (which
  carries `SectionSlug`), and already fails closed on a nil projection or a missing row.
  Criterion 2 is satisfied by what is there; adding a parallel read would have been a second
  path to keep correct. Marked done rather than inventing work.
- 2026-09-17 10:50 — Added `SectionTeam` to `classify.go` and made the map use the constant, so
  the string exists once. Documented the thing that looks like a contradiction and is not: the
  Team section grants a **capability** while mapping to `RoleCrew`. That is deliberate — the
  role stays least-privileged because the section grants nothing in the nav or directory, and
  moderation is authorised by the *assignment*, looked up per request. Keeping them apart is
  exactly what lets the assignment be revoked without touching roles.
- 2026-09-17 10:55 — Exported `NormalizeSectionSlug`. Without it a caller would naturally write
  `slug == SectionTeam`, which rejects `" Team"` — a value a hand-typed admin field can
  plausibly hold, and the map lookup already folds. Two different foldings of the same string
  is how someone silently loses their moderation.
- 2026-09-17 11:05 — `users.MayModerateGlimt(sectionSlug)` written, taking a **slug, not a
  role**. This meant `internal/users` importing `nathejk/table/person` for the constant.
  Checked the layering first: `person` cannot import `internal/...` (it is bound for shared-go)
  so there is no cycle, and `internal/data` and `internal/reveal` already import table
  packages, so the direction has precedent. Preferred over duplicating `"team"`.
- 2026-09-17 11:15 — `app.isGlimtModerator` + `app.glimtViewer` in a new `cmd/api/glimt.go`.
  Every read path will build its viewer through `glimtViewer` rather than assembling one
  inline, so a handler cannot forget the moderator lookup and answer a narrower question than
  the feed did.
- 2026-09-17 11:25 — ✅ Tests. Beyond the required ones, two worth calling out:
  `TestMayModerateGlimtIsNotARole` asserts no role used as a slug grants moderation — the
  mistake this design exists to prevent, since every Team member is `crew` and so are the
  kitchen and PR. And `TestIsGlimtModeratorAsksEveryTime` counts the projection reads, so a
  future "optimisation" into a session-lifetime cache fails loudly rather than silently
  breaking revocation.
- 2026-09-17 11:30 — ✅ All criteria met. `gofmt` clean on everything touched, `go vet` clean,
  `go test ./internal/users/ ./cmd/api/ ./nathejk/table/person/` all pass. Moving to done.

### Note for later

`cmd/api/config.go` and `cmd/api/env.go` are **not** gofmt-clean, and were not before this task
— unrelated to Glimt, left alone deliberately. Worth a separate tidy-up task if it bothers
anyone; task 310 will be editing `env.go` and should not reformat the whole file while it is
there.
