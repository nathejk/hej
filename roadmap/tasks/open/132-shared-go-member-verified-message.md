# 132 — shared-go: the member.verified message

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §8 and §11. Declare a `member.verified` style domain message in
`github.com/nathejk/shared-go` so `hej` has something to publish when a member
confirms their guardian number.

PRD 005 §11 (2026-08-25) decided that **verification is published as a domain event**,
not written to a database by the app. Two architecture rules force that: nothing writes
directly to SQL, and services do not call each other's APIs. So the fact "this member
looked at their guardian number and acknowledged it" enters the system as a message on
the log, and anyone who wants to know it consumes the same log.

The message has to be declared in shared-go rather than in `hej`, because verification
is a fact about a **member**, and members are not owned by this repo —
`types.MemberStatus` and the member read model already live in shared-go. `hej` already
depends on shared-go (`go/go.mod`, task 052), so this is a version bump, not a new
dependency.

**Payload.** Carry the member, the timestamp, and **the acknowledged number**. The
number is the non-obvious one and it is not redundant: PRD 005 §11 notes that today's
data has exactly one guardian contact per member, so "which number was acknowledged?"
looks unambiguous — but if the number on file later changes, a verification against the
old number must be invalidatable, and that is only possible if the acknowledged value
was recorded alongside the timestamp. PRD 005 §12 Q4 (editing opening up later) is the
same concern from the other side.

**Get a rough answer to PRD 005 §12 Q3 before freezing the shape.** Who eventually
consumes this — a check-in view, patrol leaders chasing their own members, a pre-event
report counting unverified members? It is out of scope to *build* that consumer, but it
decides what the payload has to carry, and the economics are asymmetric: a message is
cheap to add fields to and expensive to reshape once anything is consuming it. A short
conversation now is worth more than a correct guess.

**No `hq` work is in scope** (PRD 005 §4 and the 2026-08-30 decision in §11). Surfacing
the flag at the check-in counter is `hq`'s own board. Name the consequence honestly:
nothing downstream reacts to a verification on the day this ships.

**The two-repo release loop** from `go-bff-layout` applies. shared-go must be committed,
pushed and version-bumped before a `GOWORK=off` build in `hej` can see the new message —
a local workspace build will happily compile against an unreleased change and then fail
in CI or in the container build.

## Acceptance Criteria

- [ ] A `member.verified` style message is declared in shared-go, following the naming
      and packaging conventions of the messages already there rather than inventing a
      new one
- [ ] Payload carries: the member identity, the verification timestamp, and the
      **acknowledged contact number**
- [ ] The acknowledged number is documented in the type as the thing that makes a later
      number change invalidate the verification — not as an audit curiosity
- [ ] No member *status* value is added (PRD 005 §11 — see task 134 for the full
      reasoning); verification is a field-shaped fact
- [ ] PRD 005 §12 Q3 has at least a rough answer recorded before the shape is frozen,
      or an explicit note that it was deliberately left open and why
- [ ] shared-go is committed, pushed and tagged/version-bumped, and `hej`'s `go.mod`
      requires the new version
- [ ] `GOWORK=off go build ./...` in `hej` succeeds against the released version, not
      just a workspace build
- [ ] No changes to `hq`

## Progress Log

- 2026-08-30 — Task created from PRD 005.
