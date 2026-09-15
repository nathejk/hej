# 251 — `maphandout` projection from `qr.registered`

**Status:** open
**Priority:** high
**Created:** 2026-09-15

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

- [ ] `go/nathejk/table/maphandout/` with `table.sql`, `consumer.go`, `query.go`.
- [ ] `mapId` read through an embedding struct; documented as skan's additive field.
- [ ] A re-registration with an empty `mapId` does not erase the stored one (test).
- [ ] Re-binding the same code to the same team widens `[firstUts, lastUts]` rather than
      duplicating (test).
- [ ] A registration with no `teamId` is ignored (test).
- [ ] `ByPatrol` returns the patrol's own handouts with a `current` boolean and **no**
      successor-team fields — asserted by a test that fails if such a column is selected.
- [ ] Idempotent under replay (test).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
