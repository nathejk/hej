# 178 — Follow `team.moved` so a moved member's team is not stale

**Status:** done
**Priority:** medium
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

Discovered while implementing task 174, and deliberately left out of it.

`NathejkMemberTeamMoved` (`shared-go/messages/member.go`) records that a member now belongs to a
different team, carrying `FromTeamID` and `ToTeamID`. `hej`'s person projection consumed the event
— but only for its **status** (`racing`), ignoring both team ids.

So a moved member kept their old `teamId`, `teamName` and `teamNumber`. The consequences were
user-visible: PRD 007's patrol lookup would answer "patrol 138" *without* them while listing them
under the patrol they left; a bandit moved between klaner appeared in the wrong klan group; the
login chooser showed a stale team.

## Implementation

`go/nathejk/table/person/consumer.go` — `handleMemberTeamMoved`, dispatched ahead of the generic
lifecycle case; tests in `memberstatus_test.go`.

**Its own handler, not a branch of `handleMemberStatusChanged`.** It writes two different kinds of
fact — the status the event resolves to, and team membership — and the generic handler exists
precisely because it needs to know nothing about which event it is looking at.

**One statement, via a self-join.** `teamId` comes from the event, but `teamName` and `teamNumber`
do not: they are denormalised from *team* events, so they are copied from any sibling row already
on the destination team. Every sibling carries the same labels, which is what makes picking an
arbitrary one safe. A single statement rather than two so the move is atomic — a member briefly
holding the new `teamId` beside the old team's name would be visible in the pane, and every read
here is a single-row read with no join precisely so that cannot happen.

**`LEFT JOIN`, so an unknown destination still moves the member.** An inner join would silently
drop the move and leave them put — the worst outcome, because it looks like nothing happened.

**Empty labels rather than stale ones.** With no sibling row, `COALESCE` writes `""`. Empty is
honest and self-correcting; the old team's name is a lie that never fixes itself. The next
`patrulje.*.updated` or `.numberassigned` for the destination fills both in, because those
handlers key on `teamId` — which now includes this member. There is a test asserting that keying,
since it is what bounds the staleness window.

**The staleness does not break the lookup**, which is the reason this is acceptable rather than
merely tolerable. Task 157 resolves a typed number to a `teamId` via *any* numbered member of the
team and then lists by `teamId`, so a moved member with an empty `teamNumber` is still returned
for their new patrol. A design decision made for a different reason turns out to cover this one.

**No destination = status only.** Not an error, and not a reason to blank the team on the strength
of a missing field.

**Decided against `initialTeamId`.** hq keeps the team a member *started* with, because a survivor
moved into another patrol can still finish — with a team that is not the one they started with.
Every question this app asks is about *now*: who is out as what tonight, who is in this patrol,
whose face is this. Nothing in PRD 005 or PRD 007 asks where somebody started, and hq remains the
place to ask. Recorded at the handler so the omission reads as a decision.

## Verified against real MariaDB, not just in tests

The consumer tests assert on generated SQL strings, which cannot catch a syntax or semantics error
in a statement shape this codebase had not used before (multi-table `UPDATE ... LEFT JOIN`). So I
exercised it on a scratch table in the running `db` container:

- move with siblings present → member lands on `team-2` with `Patrulje Ulvene` / `139`, and
  multiple sibling rows cause no trouble (no "subquery returns more than one row" class of error);
- move to an unknown team → lands with empty name and number, as intended;
- the destination team's later update → fills both in, confirming the self-correction.

Scratch table dropped afterwards; the live `person` table was not touched. Note it does not yet
have the `teamNumber` column — this deployment predates task 176's migration and will pick it up
via `EnsureColumn` on the next API restart.

## Acceptance Criteria

- [x] `team.moved` updates the member's `teamId` alongside the status.
- [x] `teamName` and `teamNumber` end up consistent with the destination team, or the staleness
      window is documented and bounded — copied from a sibling when one exists; otherwise empty
      and bounded by the next team event, verified against the database.
- [x] A member moved to a team this projection has not yet seen ends up correct once that team's
      events arrive.
- [x] Decide, and record, whether an `initialTeamId` is needed here at all — decided no, with
      reasoning at the handler.
- [x] Tests: move within a known team, move to an unknown team, then the team's events arriving
      afterwards; and a replay producing the same result as the live stream
      (`TestTeamMovedIsIdempotent`).
- [x] `TestTeamMovedDoesNotChangeTheTeam` is replaced rather than deleted quietly — replaced by
      `TestTeamMovedMovesTheMemberAndCopiesTheTeamLabels`, which names it so the history is
      findable.

## Progress Log

- 2026-09-01 11:45 — Picked up. Gave `team.moved` its own handler, dispatched before the generic
  lifecycle case.
- 2026-09-01 11:55 — Chose a single joined UPDATE over two statements, so a reader can never catch
  the row mid-move with a new team id and an old team name.
- 2026-09-01 12:05 — Realised the bounded staleness is harmless for the lookup specifically,
  because task 157 matches on `teamId` rather than on the number. Added a test pinning that the
  team handlers key on `teamId`, since that is what closes the window.
- 2026-09-01 12:15 — Verified the new statement shape against the real MariaDB on a scratch table:
  siblings present, unknown destination, and the later fill-in. Statement-string tests could not
  have caught a syntax error here.
- 2026-09-01 12:20 — ✅ All criteria met. `go build`, `go vet`, `go test ./...` clean; `gofmt`
  clean.
