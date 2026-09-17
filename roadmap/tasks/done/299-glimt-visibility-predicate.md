# 299 — Glimt visibility predicate in internal/users

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

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

- [x] `users.GlimtGroup(role)` maps all seven roles, with a test naming each
- [x] `users.MaySeeGlimt(...)` covers public / nathejk / group / hidden, with the moderator override inside it
- [x] Table-driven test asserts a spejder cannot see a crew `group` glimt, and vice versa
- [x] Test asserts a moderator sees a `group` glimt from a group they are not in
- [x] Test asserts a hidden glimt is invisible to a normal member of the right group
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 09:10 — Picked up. Read `internal/users/contacts.go` and `grouping.go` first; this
  package already has exactly the shape PRD 019 asks for (one file that is the only place a
  visibility question is answered, with the reasoning inline), so `glimt.go` follows it rather
  than inventing a second style.
- 2026-09-17 09:25 — Wrote `GlimtGroup` + `GlimtGroupFor`. Noted a real subtlety worth
  recording: this puts **gøgler in the crew bucket**, where `Population` deliberately keeps
  them apart. That is not an oversight — the directory separates them because listing a gøgler
  among crew misrepresents who someone is, while a glimt shared "with my group" by a gøgler is
  meant for the people staffing the event alongside them. Same words, different question; both
  the doc comment and a test now say so.
- 2026-09-17 09:35 — `MaySeeGlimt` written with the moderator override **inside** the
  predicate, per PRD 019 §8. Order is deliberate: moderator, then own, then hidden, then
  audience. Hidden is checked before the audience because hiding has to beat every audience —
  including `public`, which has no approval queue in front of it.
- 2026-09-17 09:40 — Decision: `MaySeeGlimt` does **not** model anonymous callers. The public
  page queries `audience = public AND NOT hidden` directly and ignores the session cookie
  (task 323), so there is no viewer to pass and no way for a zero-valued `GlimtViewer` to
  satisfy a rule by accident.
- 2026-09-17 09:55 — Added `GlimtFeedFilter` after realising the predicate alone cannot serve a
  feed: it needs rows before it can judge them, and fetching an event's worth of glimt to keep
  three of them is not viable after the race. The filter is a narrowing of the predicate, and
  every row still passes through `MaySeeGlimt` on the way out.
- 2026-09-17 10:05 — Replaced a first draft of that helper (`GlimtAudiencesVisibleTo`) which
  returned every audience unconditionally and was therefore useless — it encoded no rule.
- 2026-09-17 10:15 — ✅ The agreement test earned its place immediately. Enumerating every
  viewer × subject combination found a genuine divergence on the first run: for an
  **unrecognised role**, `MaySeeGlimt` fails closed, but the filter has no role to validate, so
  it still matched the viewer's own rows and every `public` row. Fixed by adding an explicit
  `Denied` field rather than relying on "no group and no person id", since that is also what a
  legitimately group-less viewer looks like and the difference decides between own-rows and
  no-rows.
- 2026-09-17 10:20 — ✅ All criteria met. `gofmt` clean, `go vet` clean,
  `go test ./internal/users/` and the full `go test ./...` pass. Moving to done.

### Notes for the tasks that build on this

- `GlimtViewer.IsModerator` is **passed in, never derived here**. It comes from task 300's
  per-request section lookup precisely so that revoking the Team assignment revokes the power.
  This package must not learn to answer it from a `Role` or a session claim.
- Task 305 (media handler) must call `MaySeeGlimt`, not a looser check. The test named in that
  task — a `group`-scoped ref returning 403 for an outsider — is the one that proves a blob URL
  is not a bearer token.
- Task 304's feed query should build its WHERE clause from `GlimtFeedFilterFor` and keep
  `filter.Matches` / `MaySeeGlimt` on the way out. If someone optimises the SQL later, the
  agreement test is what tells them they broke it.
