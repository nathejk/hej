# 150 — Decide how member status reaches the contacts pane

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

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

## Decision

**Option 2 — lift the transitions to `shared-go` and consume them here** — with an
important correction to the PRD's ranking, and a smaller scope than expected.

**Option 1 is not actually available.** PRD 007 §8 ranked "read `hq`'s projection at
lookup time" first. Investigation says otherwise: `hej` has no `hq` client, no
credentials for `hq`'s database, and no HTTP dependency on it. This service is
event-sourced from the stream (`go/nathejk/table/person/consumer.go`) and its own
projection is the only read model it has. Ranking that option first was a mistake made
without checking; PRD 007 §8 has been corrected.

**Most of option 2 already exists**, which makes it much cheaper than the PRD assumed.
`shared-go/types/member.go` **already defines the whole lifecycle vocabulary** —
`registered`, `seated`, `racing`, `finished`, `waiting`, `transit`, `sheltered`,
`reunited`, `released` — with substantial documentation and helpers (`Valid()`,
`CanFinish()`, `InOurCare()`). The shared vocabulary is not the missing piece.

What is missing is narrower:

- the **message types for the transitions**, which per the `consumer.go` comment are
  still defined in `hq` — these need lifting to `shared-go` (task 174, precedent
  task 147);
- a predicate for "has this member left the race", which belongs next to `InOurCare()`
  rather than in this repo (task 175).

**Why this is not the parallel projection the comment warns against.** The warning is
about a *second notion* of status — a different vocabulary, or rules re-derived
locally, which could then disagree with `hq`. Storing `shared-go`'s
`types.MemberStatus` verbatim, from the same events, is a **cache of one shared notion**,
not a second one. The line to hold: `hej` may store the value and read it, but must not
implement lifecycle *rules* (which transitions are legal, what counts as finished).
Those stay in `shared-go`.

**The split, as required:**

- **Live patrol lookup** (task 157) — reads the full current `types.MemberStatus` from
  the projection and returns it. Fresh because the lookup is online.
- **Cached directory manifest** (task 154) — carries only a coarse boolean
  `stillInRace`, derived in exactly one place. One bit, not a lifecycle, because that is
  all the directory needs to mark a withdrawn colleague.

**Interim, so tasks 154 and 157 are not blocked:** the coarse flag is derived in a single
helper in `hej` from `reunited` / `released`, with a comment pointing at task 175 to move
the predicate into `shared-go`. Until task 174 lands, no transition events arrive, so the
flag reads `true` for everyone — which is correct for a pre-event state and must be
documented at the call site rather than silently assumed.

**Not chosen:** option 3. No new status column, and no locally-invented status strings.

## Acceptance Criteria

- [x] Option chosen and written up in this file, with the reasoning.
- [x] PRD 007 §8 updated to record the decision.
- [x] The split is explicit: what the live lookup reads vs. what the manifest carries.
- [x] If option 2, a follow-up task is created for the `shared-go` lift and referenced
      here — tasks **174** (lift transition message types) and **175** (`Ended()`
      predicate next to `InOurCare()`).
- [x] No new status column is added to `person` unless option 3 is explicitly chosen
      and justified — none added.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8 / §11.6.
- 2026-08-31 10:05 — Picked up. Plan: check what `hq` access exists, what `shared-go`
  already carries, then choose.
- 2026-08-31 10:20 — Found `shared-go/types/member.go` already defines the full
  `MemberStatus` vocabulary with docs and `Valid`/`CanFinish`/`InOurCare`. Option 2 is
  much cheaper than the PRD assumed.
- 2026-08-31 10:30 — Found option 1 is not available at all: no `hq` client, no shared
  database, `hej` is event-sourced. The PRD's ranking was written without checking.
  Correcting the PRD rather than quietly picking something else.
- 2026-08-31 10:40 — Confirmed the transition *message types* live in `hq`, not
  `shared-go` (`shared-go/tables/spejder/consumer.go` handles only
  updated/deleted/started). Created tasks 174 and 175 for the lift.
- 2026-08-31 10:50 — Decision recorded: option 2, with the live/coarse split, and an
  interim single-place derivation so 154 and 157 are unblocked. PRD 007 §8 updated.
