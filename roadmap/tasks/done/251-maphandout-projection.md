# 251 — `maphandout` projection from `qr.registered`

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

PRD 016 phase 1. Record which map sheets have been handed to which team, so a patrol can
see its own list.

**Subject: `NATHEJK.*.qr.*.registered`.** Note the trap (PRD 016 §8): `qr.registered`
binds a printed code and its sheet to a team — that *is* the handover — while
`qr.scanned` is a scan of the team's code at a post. Consuming the wrong one produces a
handout list that grows at every checkpoint. `qr.scanned` belongs to task 254.

Shape, copied from hq's `maphandout` (narrowed — see below):

- Keyed `(year, qrId, teamId)`, **not** `(year, qrId)`. One row per team a code was ever
  bound to, because a sheet legitimately changes hands: when a team is discontinued its
  scouts and their sheets are reassigned. Keying on the code alone would answer "who holds
  it now" but not "what has this team been given", which is the question the app asks.
- `mapId` arrives as an **additive JSON field skan adds to the shared body** — it is not
  on `messages.NathejkQrRegistered`, so read it through a struct embedding that type.
- `mapId` is **never overwritten with `""`**: empty means *unknown sheet*, not *no sheet*,
  and a re-scan before the sheet was recorded must not erase it.
- `firstUts` / `lastUts` in **unix seconds** (widen with LEAST/GREATEST so replay order
  does not matter).
- A binding with no team is not a handout to anyone — skip it.

**Narrower than hq's on purpose:** we need "does this patrol still hold this sheet", never
who else holds it. The successor team's id, number and name must not be selectable, let
alone returned (PRD 016 §6). That is the single clearest reason this projection is a copy
rather than shared code.

## Acceptance Criteria

- [x] `go/nathejk/table/maphandout/` with `table.sql`, `consumer.go`, `query.go`.
- [x] `mapId` read through an embedding struct; documented as skan's additive field.
- [x] A re-registration with an empty `mapId` does not erase the stored one (test).
- [x] Re-binding the same code to the same team widens `[firstUts, lastUts]` rather than
      duplicating (test).
- [x] A registration with no `teamId` is ignored (test).
- [x] `ByPatrol` returns the patrol's own handouts with a `current` boolean and **no**
      successor-team fields — asserted by a test that fails if such a column is selected.
- [x] Idempotent under replay (test).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: `go/nathejk/table/maphandout/` — schema keyed
  `(year, qrId, teamId)`, consumer on `qr.*.registered` reading `mapId` through a struct embedding
  `messages.NathejkQrRegistered`, and a `ByPatrol` read that derives "still held" without ever
  selecting the successor team.
- 2026-09-15 — Consumer written. `mapId=IF(VALUES(mapId) = '', mapId, VALUES(mapId))` decided in
  SQL rather than Go, because the comparison is against the *stored* value, which the process
  folding the event does not have.
- 2026-09-15 — Decision: **extracted the read into a package-level `byPatrolQuery` constant.** Not
  for tidiness — the guarantee this projection makes is about a column that is *absent*, and no
  assertion on a result set can show the absence of a column nobody has added yet. The constant
  gives the test something to assert against, so
  `TestByPatrolNeverSelectsTheSuccessorTeam` fails the day someone copies a `JOIN patrulje` across
  from hq.
- 2026-09-15 — Narrowed against hq deliberately: `Handout` has `Current bool` where hq's has
  `CurrentTeamID/Number/Name`. The successor's identity is used *inside* the window function and
  never leaves it.
- 2026-09-15 — Blocker (self-inflicted), resolved: my first `TestMissingYearIsAnError` asserted an
  error for the subject `NATHEJK`, and it failed — correctly. The handler re-checks the subscribed
  pattern first, and a year-less subject cannot match a pattern that requires the year segment, so
  the year guard is unreachable through the subscription. Rewrote it as two honest tests: an
  unrelated subject (`qr.scanned`) is ignored without writing, and `subjectYear` is tested directly.
  The guard stays for the day the pattern is widened.
- 2026-09-15 — ✅ All criteria complete. 11 tests; `go build ./...`, `go vet ./...`, `gofmt` and the
  full suite clean.
- 2026-09-15 — Note: `TestOnlyRegisteredIsConsumed` pins the registered-vs-scanned distinction at
  the subscription level, since consuming `qr.scanned` here would make the handout list grow at every
  checkpoint — wrong in a way that looks plausible for about one race.
- 2026-09-15 — Done. Moving to done/; tasks 252–254 remain in phase 1.
