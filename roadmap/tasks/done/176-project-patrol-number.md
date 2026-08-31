# 176 — Project the patrol number onto the person row

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

Split out of task 157, which cannot be built without it.

PRD 007's patrol lookup takes a **patrol number** ("slå patrulje 138 op"). The person
projection has no such field: it carries `teamId` (opaque) and `teamName` (a label like
"Patrulje Ravnene"). There is nothing to match a typed number against.

Upstream has it. `shared-go/messages/team.go` defines
`NathejkPatrolNumberAssigned{TeamID, TeamNumber}`, published on
`NATHEJK.<year>.patrulje.<teamId>.numberassigned`
(`shared-go/tables/patrulje/commands.go`), and shared-go's own `patrulje` table projects
it. So this is a local projection gap, not a cross-repo lift — no dependency on tasks
174/175.

**Follow the `teamName` pattern exactly** (`handleTeamUpdated` in
`go/nathejk/table/person/consumer.go`): denormalise onto every person of that team with an
`UPDATE ... WHERE teamId = ? AND year = ?`, and **no INSERT** — members arrive on their own
events, and a team event must not invent people.

## Implementation

- `table.sql` + `table.go`: `teamNumber VARCHAR(32) NOT NULL DEFAULT ""`, added through the
  existing `EnsureColumn` list so deployed databases get it without a recreate, plus a
  `year_teamnumber` index — the lookup's only query shape is one exact number within a year,
  and it is an interactive request during the race.
- `querier.go`: `TeamNumber` on `Person`, added to `personColumns` and `scanPerson`.
- `consumer.go`: `NATHEJK:*.patrulje.*.numberassigned` consumed, handled by
  `handlePatrolNumberAssigned`.
- `patrolnumber_test.go`: denormalisation, subject fallback, empty number, empty team,
  idempotence on replay, and renumbering.

A string, not an integer, matching upstream — shared-go stores it in a VARCHAR and picks the
next one with `ORDER BY length(teamNumber), teamNumber`. Parsing it here would be a second
opinion about the format.

The test helper `personColumnNames()` absorbed the new column for free, exactly as its
comment predicted ("a new column costs nothing here"). Pleasant to see that pay off — the
only edit needed was the column name itself.

## The ordering gap — documented, not solved

The acceptance criteria allowed either fixing this or recording it. Recording it, with a
mitigation that lands in task 157.

`handlePatrolNumberAssigned` covers "number assigned after the member was projected". The
reverse is not covered: **`NathejkTeamUpdated` carries no team number** — I checked the
message, which has `Name`, `GroupName`, `AdvspejdNumber` and contact fields but no
`TeamNumber` — so a member who signs up *after* their patrulje was numbered keeps an empty
`teamNumber` until the team is renumbered.

I did not solve it with a self-join backfill, because **task 157's lookup makes it moot**:
the lookup resolves the typed number to a `teamId` via any one numbered member, then lists
by `teamId`. A late member is therefore still found. That is a better fix than a backfill —
one query shape instead of a write-path workaround, and it cannot drift out of sync.

**Residual, recorded honestly:** a patrulje where *no* member carries the number would be
unfindable by number. That requires every member to have signed up after numbering, which
should not happen in practice, but it is a real hole rather than an impossible one. If it
turns out to occur, the fix is a backfill on member sign-up, and this is where to look.

## Acceptance Criteria

- [x] `person` gains a `teamNumber` column, defaulting to empty.
- [x] The consumer handles `NATHEJK.*.patrulje.*.numberassigned` and denormalises the
      number onto every member of that team, UPDATE-only.
- [x] `person.Person` exposes `TeamNumber`, and it is part of `personColumns`.
- [~] A member projected *after* the number is assigned still ends up with the number, **or
      the gap is documented** — documented above, with the mitigation in task 157.
- [x] Stored as a string, matching upstream.
- [x] Replay-safe: `TestPatrolNumberAssignedIsIdempotent` asserts a replayed event produces
      an identical statement.
- [x] Tests cover: number assigned then member added, member added then number assigned
      (as the documented gap), and a number assigned for a team with no members (an UPDATE
      that matches nothing, asserted to contain no INSERT).

## Progress Log

- 2026-08-31 18:35 — Picked up. Plan: follow the `teamName` denormalisation pattern.
- 2026-08-31 18:45 — Column, index, `Person` field and `personColumns` done. Existing
  querier tests failed on the scan arity until `personColumnNames()` learned the new name —
  its by-name design meant that was the only edit.
- 2026-08-31 18:55 — Consumer handler and tests written, including replay idempotence.
- 2026-08-31 19:05 — Checked whether `NathejkTeamUpdated` could close the ordering gap. It
  carries no team number, so it cannot. Corrected a comment I had written claiming the
  number is reassigned on every patrulje update — it is not.
- 2026-08-31 19:10 — Decided against a self-join backfill: task 157's lookup can resolve
  number → teamId → members, which handles late members without a write-path workaround.
  Residual (a patrulje with no numbered member at all) recorded above.
- 2026-08-31 19:15 — ✅ Criteria met. `go vet ./...` and `go test ./...` pass; `gofmt` clean
  on touched files. Task 157 is now unblocked.
