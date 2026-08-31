# 150 — Decide how member status reaches the contacts pane

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

PRD 007 requires a **clear status marking** for members who leave the race
(`released` / `reunited`), and the patrol lookup returns each member's current status.
This repo does not have that data.

`person.memberStatus` is `racing` or empty, and the comment above `handleTeamStarted`
in `go/nathejk/table/person/consumer.go` explains why: `hq` already projects the full
lifecycle (`spejderstatus`), and mirroring it here would create "a second, lagging
notion of member status in a second repo, disagreeing with `hq` in ways nobody would
notice until an organizer compared two screens". It names the sanctioned routes: read
`hq`'s projection, or lift it to `shared-go` — **not** grow a parallel one here.

This is a **decision task**, and it blocks task 154 (manifest) because it changes what
that endpoint reads. PRD 007 §8 "Member status" ranks the options:

1. Read `hq`'s projection at lookup time — cheapest and most correct now that the
   patrol lookup is uncached and online.
2. Lift the lifecycle projection to `shared-go` (precedent: task 147).
3. Project a parallel status into `person` — the option the comment warns against;
   listed so nobody arrives at it by accident.

Note the two needs are different: the **lookup** needs full current status live, while
the **cached directory** needs only a coarse "still in the race" flag per person.

## Acceptance Criteria

- [ ] Option chosen and written up in this file, with the reasoning.
- [ ] PRD 007 §8 updated to record the decision.
- [ ] The split is explicit: what the live lookup reads vs. what the manifest carries.
- [ ] If option 2, a follow-up task is created for the `shared-go` lift and referenced
      here.
- [ ] No new status column is added to `person` unless option 3 is explicitly chosen
      and justified.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8 / §11.6.
