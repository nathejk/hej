# 153 — Server-supplied grouping model for the contacts directory

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

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

## Implementation

`go/internal/users/grouping.go` + `grouping_test.go`.

`GroupPathFor(viewer, subject, population) []Group`, where `Group` is `{ID, Label, IsOwn}`.

**A path, not a group.** Every path has exactly one element today. Returning a slice is
the cheap way to absorb the subsection tier: `lok → klan` becomes a two-element path and
the client's rendering loop is unchanged, whereas a single `group` field would have to be
replaced at every read site. This is the whole reason the task exists, so it is worth the
mild awkwardness now.

**Grouping is per viewer, and takes a population.** Falls out of task 152: a crew bandit
is listed in two populations, so "which group are they in" has no viewer-independent
answer. A bandit sees them in the shared klan; a crew member sees them under Crew. Tested
explicitly.

**`GroupPathFor` re-checks permission and returns `nil` when the viewer may not list that
population.** Slightly redundant, since callers check `MayList` first — but a grouping
function that cheerfully labels a spejder for a bandit is one refactor away from being the
leak, and returning nil means a caller who forgets cannot render anything.

**`IsOwn` is deliberately false for the no-klan group.** Two banditter both missing klan
data are not colleagues, and auto-expanding a bucket of orphaned records would present a
data problem as a group.

Labels are Danish and live server-side ("Gøglere", "Crew", "Uden klan"), consistent with
grouping being data rather than presentation. Group ids are stable and opaque, so
expansion state survives an upstream label edit.

## Acceptance Criteria

- [x] A group type carrying id, label and own-group flag, produced server-side.
- [x] Bandit grouping derives from klan; gøgler and crew are single groups.
- [x] A person with no klan lands in one clearly-labelled "Uden klan" group rather than
      disappearing.
- [x] The shape does not preclude a second grouping tier (documented in a comment).
- [x] Tests cover own-group flagging per role, and the no-klan fallback.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8.
- 2026-08-31 13:10 — Picked up. Plan: `GroupPathFor` in `internal/users`, next to
  placement and permission.
- 2026-08-31 13:20 — Chose a `[]Group` path over a single group, specifically so the
  expected lok/subsection tier is additive. Documented at the type.
- 2026-08-31 13:35 — Made grouping take a population and re-check `MayList`, returning nil
  on refusal — grouping must not become a path around authorization.
- 2026-08-31 13:45 — Decided the no-klan group is never `IsOwn`: shared missing data is not
  a shared klan.
- 2026-08-31 13:55 — ✅ All criteria met. `go test ./internal/users/` passes, `go vet` and
  `gofmt` clean.
