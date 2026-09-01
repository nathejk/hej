# 178 — Follow `team.moved` so a moved member's team is not stale

**Status:** open
**Priority:** medium
**Created:** 2026-09-01

## Description

Discovered while implementing task 174, and deliberately left out of it.

`NathejkMemberTeamMoved` (`shared-go/messages/member.go`) records that a member now belongs to a
different team, carrying `FromTeamID` and `ToTeamID`. `hej`'s person projection consumes the event
— but only for its **status** (`racing`), ignoring both team ids.

So a moved member keeps their old `teamId`, `teamName` and `teamNumber` in this projection. The
consequences are user-visible:

- **PRD 007's patrol lookup shows them under the wrong patrol.** A samarit sent to patrol 138 for
  a member who was moved into it would not find them, and would find them listed in the patrol
  they left. For a safety surface that is the wrong kind of wrong.
- A bandit moved between klaner appears in the wrong klan group in the contacts directory.
- The login chooser (task 079) uses team name to tell two people on one phone apart, so it would
  show a stale team.

## Why it was not fixed in task 174

Task 174's job was the status, and the write is a one-line `UPDATE`. Following team membership is a
different question with real design content, and bundling them would have produced a change nobody
could review as one thing:

- **Three columns, not one.** `teamId` comes from the event, but `teamName` and `teamNumber` are
  denormalised from *team* events (`handleTeamUpdated`, `handlePatrolNumberAssigned`). Moving a
  member means either copying those from a sibling row on the destination team, or accepting that
  they are briefly stale until the next team event.
- **Ordering.** A member may be moved to a team this projection has not seen yet, in which case
  there is no sibling row to copy from.
- **`initialTeamId`.** hq's projection keeps the team a member *started* with, because a survivor
  moved into another patrol can still finish, with a team that is not the one they started with.
  This projection has no such column and may need one — or may not, since PRD 007 asks "who is out
  as what tonight", not "who started where".
- **Scope discipline.** Task 150's decision is that `hej` caches upstream facts rather than
  growing a parallel lifecycle. Team membership is arguably the same kind of fact as status, but
  it is worth saying so explicitly rather than drifting into it.

## Acceptance Criteria

- [ ] `team.moved` updates the member's `teamId` alongside the status.
- [ ] `teamName` and `teamNumber` end up consistent with the destination team, or the staleness
      window is documented and bounded.
- [ ] A member moved to a team this projection has not yet seen ends up correct once that team's
      events arrive.
- [ ] Decide, and record, whether an `initialTeamId` is needed here at all.
- [ ] Tests: move within a known team, move to an unknown team, then the team's events arriving
      afterwards; and a replay producing the same result as the live stream.
- [ ] `TestTeamMovedDoesNotChangeTheTeam` (which pins today's deliberate gap) is replaced rather
      than deleted quietly.

## Progress Log

- 2026-09-01 — Task created from a gap found while implementing task 174, with the reasoning for
  not fixing it there recorded above.
