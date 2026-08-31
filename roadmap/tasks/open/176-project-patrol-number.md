# 176 — Project the patrol number onto the person row

**Status:** open
**Priority:** high
**Created:** 2026-08-31

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
events, and a team event must not invent people. The existing comment explains the
reasoning; the same applies here.

Two details worth getting right:

- **Order-independence.** The number may be assigned before or after a member's own event
  is projected. `teamName` has the same problem and solves it by being an idempotent
  UPDATE keyed on `teamId`; a member arriving later needs the number backfilled from the
  team's row, so check whether the existing sign-up handlers already read team state, and
  if not, note the gap rather than papering over it.
- **The number is a string, not an int.** shared-go stores and increments it as a string
  (`teamNumber VARCHAR`), and its "next number" query orders by `length(teamNumber),
  teamNumber`. Storing it as an integer here would be a second opinion about the format.

## Acceptance Criteria

- [ ] `person` gains a `teamNumber` column, defaulting to empty.
- [ ] The consumer handles `NATHEJK.*.patrulje.*.numberassigned` and denormalises the
      number onto every member of that team, UPDATE-only.
- [ ] `person.Person` exposes `TeamNumber`, and it is part of `personColumns`.
- [ ] A member projected *after* the number is assigned still ends up with the number, or
      the gap is documented explicitly in the task log if it needs a separate fix.
- [ ] Stored as a string, matching upstream.
- [ ] Replay-safe: a full replay produces the same numbers as the live stream.
- [ ] Tests cover: number assigned then member added, member added then number assigned,
      and a number assigned for a team with no members (must not create rows).

## Progress Log

- 2026-08-31 — Task created, split out of task 157 after finding the person projection has
  no patrol number to match against.
