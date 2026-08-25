# 072 — Project spejder (and patrulje team names)

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 006 §8. Consume `NATHEJK.*.spejder.*.updated` / `.deleted` and the patrulje
subjects (`signedup`, `updated`, `started`) into the person projection.

Spejder are the only population with a **guardian phone** (`PhoneParent`), which
PRD 005's confirmation step depends on — the model must make "not applicable"
distinguishable from "missing".

Consume events directly rather than reading shared-go's `spejder` projection: the
survey found `spejder.GetByID` is a stub returning `nil, nil` and `GetAll` joins
`spejderstatus`, which shared-go does not own.

## Acceptance Criteria

- [x] `Consumes()` lists the spejder + patrulje subjects
- [x] `HandleMessage` is idempotent (replayed on every boot)
- [x] Guardian phone stored, and "not applicable" distinguishable from "missing"
- [x] Team id and name populated from the patrulje events
- [ ] Member status carried (PRD 005's skip rule reads it) — **deferred, see log**
- [x] Tests using `cqrstest` fakes, no database required

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-25 — Picked up. Read shared-go's own spejder projector for the subject names
  and message shapes rather than guessing them.
- 2026-08-25 — Learned two things worth writing down: the guardian number arrives as
  `NathejkScoutUpdated.PhoneContact` (not `PhoneParent`), and the team link arrives on
  the *older* `NathejkMemberAdded` shape carried on the same event — shared-go reads
  both bodies from one message for exactly that reason, so this projector does too.
- 2026-08-25 — Note the two subject separators: member events use
  `NATHEJK.*.spejder.*` and team events `NATHEJK:*.patrulje.*`. Copied verbatim from
  the projectors known to receive them, because a subject that does not match is
  **silent** — the projection just stays empty.
- 2026-08-25 — `cqrs.Writer` takes a finished statement, not statement+args, so
  escaping is this projector's job. Added `quote`/`nullableQuote`/`upsert` helpers in
  `sql.go` so no call site formats a value by hand, plus a test that a name containing
  quotes cannot terminate the literal.
- 2026-08-25 — `upsert` writes **only the columns an event carries**, which is what
  lets the spejder handler and the patrulje handler cooperate on one row without
  resetting each other's work. Team name is denormalized onto each person so the login
  path reads one row with no join; the cost is a fan-out on write, and teams are renamed
  far less often than members log in.
- 2026-08-25 — Deletes and team updates are `UPDATE`-only, no insert: a delete for
  someone never seen is a no-op, and a team event must not invent people.
- 2026-08-25 — ✅ 13 unit tests via `cqrstest`, all green.

### Verified against the real event stream — and it found a bug

- 2026-08-25 — With Docker back, the dev stack connects to the **shared** broker, which
  holds real historical Nathejk events. So the replay projected real data immediately.
- 2026-08-25 — First run: **6529 dead letters**, all one error —
  `Incorrect date value: '2010-06-03T22:00:00.000Z' for column birthday`.
  `types.Date` documents itself as `2006-01-02`, but the real stream carries RFC3339
  timestamps. shared-go never noticed because its `birthday` column is `VARCHAR(99)`;
  this projection uses `DATE`, which rejected them.
- 2026-08-25 — The subtle part, and the reason not to just truncate to ten characters:
  the values are **midnight Copenhagen expressed in UTC** — `22:00Z` in summer (UTC+2),
  `23:00Z` in winter (UTC+1), exactly as the dead letters showed. Truncating would have
  recorded **the day before** each member's real birthday. `parseBirthday` converts via
  `Europe/Copenhagen` instead, and `cmd/api` now embeds `time/tzdata` because the prod
  image is bare alpine with no system zoneinfo.
- 2026-08-25 — Second fix in the same place: an unparseable birthday now **omits the
  column** rather than failing the statement. Before, one bad date dead-lettered the
  entire row — costing that member their login over a field nothing authenticates on.
- 2026-08-25 — After the fix, truncate + full replay: **1727 persons, 0 dead letters**
  (1610 with a phone, 1337 with a guardian number, 1643 with a birthday, 1496 with a
  team name, across 2 years).
- 2026-08-25 — Normalization proven on real data: **1609 of 1610** non-empty phones are
  `+45…` and **zero** are unnormalized (the remaining one is a non-Danish number). So
  the shared-normalizer design from task 070 holds up outside its own tests.
- 2026-08-25 — **Deferred:** `memberStatus`. It does not arrive on these events — it
  comes from the `spejderstatus`/lifecycle subjects, which are a separate family. The
  column exists and stays empty; PRD 005's skip rule needs a follow-up task rather than
  a guess here. Criterion left unchecked rather than quietly claimed.
- 2026-08-25 — Note for whoever runs the stack: replaying real events onto a dev
  machine puts **real minors' names, addresses and guardian numbers** in the local
  database automatically. That is a live privacy concern and it directly contradicts
  task 065's constraint; raised there.
- 2026-08-25 — Moving to done.
