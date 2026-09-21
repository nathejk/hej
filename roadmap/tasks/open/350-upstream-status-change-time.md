# 350 — Ask upstream to carry the time a member's status changed

**Status:** open
**Priority:** low
**Created:** 2026-09-21
**Picked up by:**
**Started:**
**Completed:**

## Description

Follow-up from task 349, and a **measured** finding rather than a suspicion.

PRD 011 §0b.6 needs to know *when* a member left the race, so the post-race distance estimate can stop
counting their recorded points from that moment instead of discarding their whole night. Task 349 projects
`person.memberStatusAt` from `msg.Time()` — the stream time — and that is the only source available, because
the lifecycle event bodies in `shared-go/messages/member.go` carry **no timestamp field**:
`NathejkMemberStatusOverridden`, `NathejkMemberHandoverCompleted` and the rest have a member id, a team id, a
status and an actor.

**And in the 2026 dev data, those events arrive with a zero stream time.** Measured after task 349 shipped:

| status | rows | with a time |
|---|---|---|
| `racing` (from `patrulje…started`) | 85 | 73 |
| `waiting`, `transit`, `sheltered`, `reunited`, `released` | 5 | **0** |

So team events and `qr.scanned` carry a usable `msg.Time()` (the `scan` projection depends on it, and its
`uts` values are correct), while the **member lifecycle events do not**. The 12 `racing` rows without a time
fit the same pattern: those came from `withdrawal.cancelled` rather than from the start event.

**The consequence today:** every withdrawal has an unknown time, so task 349's rule degrades to its
documented fallback — a withdrawn member's points are excluded *entirely* rather than from when they left.
That is the safe direction (it can only lower a figure labelled *mindst*) and it costs a patrol the
kilometres that member genuinely walked before withdrawing.

Two ways to fix it, and the first is better:

1. **A timestamp in the event body**, next to the status, in `shared-go/messages/member.go`. A withdrawal
   *happened at a time*, and that is a fact about the event rather than about the transport — which is also
   why relying on the stream time was always the weaker option: a replay into a new broker, a re-publish, or
   a migration can change it, and none of those should move when a child left the race.
2. **Set the message time when publishing** in whichever service emits these. Cheaper, and it fixes only new
   events — the history stays blank.

Note this is the same class of question as task 175 (`MemberStatus` predicates belong in shared-go): the fact
is the organizers', and two repos guessing at it is how they come to disagree.

## Acceptance Criteria

- [ ] Whether a lifecycle event may carry its own timestamp is decided with whoever owns `hq`.
- [ ] If yes: the field is added to `shared-go/messages/member.go` with documentation, and the `person`
      consumer prefers the body's time over `msg.Time()`.
- [ ] If no: the publisher sets the message time, and this task records that the history remains unstamped.
- [ ] `hej`'s fallback ("no time → exclude the member's points entirely") stays in place either way — it is
      the correct reading of "we do not know when", not a workaround to be removed.
- [ ] Re-measure the table above afterwards, so the fix is shown to have landed rather than assumed.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-21 — Task created from task 349's dev-stack verification, which is what turned this from an
  assumption into a number.
