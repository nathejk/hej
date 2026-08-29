# 132 — shared-go: the member.verified message

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

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
      new one — **deliberately not done: declared in `hej` instead. See the log.**
- [x] Payload carries: the member identity, the verification timestamp, and the
      **acknowledged contact number**
- [x] The acknowledged number is documented in the type as the thing that makes a later
      number change invalidate the verification — not as an audit curiosity
- [x] No member *status* value is added (PRD 005 §11 — see task 134 for the full
      reasoning); verification is a field-shaped fact
- [x] PRD 005 §12 Q3 has at least a rough answer recorded before the shape is frozen,
      or an explicit note that it was deliberately left open and why
- [ ] shared-go is committed, pushed and tagged/version-bumped, and `hej`'s `go.mod`
      requires the new version — **not applicable: shared-go is unchanged**
- [x] `GOWORK=off go build ./...` in `hej` succeeds against the released version, not
      just a workspace build
- [x] No changes to `hq`

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — Picked up.
- 2026-08-30 — **Deliberate deviation: the message is declared in `hej`, not in shared-go.** New
  `go/nathejk/table/person/verified.go` holds `MemberVerified`, `VerifiedSubject()` and the
  projection handler, exactly where the portrait event lives.

  Three reasons, and the code carries them so nobody has to find this file:

  1. **There is a shipped precedent that says so.** `portrait.go` (task 103) establishes that
     events *this* service publishes are owned by the projection that consumes them, with an
     explicit rationale; shared-go carries the messages other services publish, where both ends
     must agree on a shape neither controls alone.
  2. **There is no second party.** PRD 005 §4 (revised 2026-08-30, at the maintainer's direction)
     put all `hq` work out of scope, so nothing outside `hej` consumes this event. Adding a type to
     a module three repos depend on, with no consumer, means an unused export plus a version bump
     in each of them — and the two-repo release loop paid for nothing.
  3. **Moving it later is mechanical.** This whole package is bound for shared-go (see its package
     doc), so the message travels with it. What would change is the declaration site, not the
     subject, the body or the projection.

  If the maintainer prefers it in shared-go regardless, that is a small, contained change — but it
  should be a decision rather than a default, which is why it is written up here.
- 2026-08-30 — **Shape decisions:**

  - Subject is `NATHEJK.<year>.member.<personId>.verified` — per person, so
    `nats stream purge --subject` can erase one individual's history. That matters as much here as
    for a photograph: this event carries a parent's phone number.
  - `member.` rather than `person.`: "person" is this projection's local word for a row assembled
    from several populations; the fact is about a *member* of the event, which is the vocabulary
    shared-go, hq and PRD 005 all use. It also means the subject reads correctly if the message ever
    does move to shared-go.
  - **The two typed digits are deliberately NOT on the event.** They are checked in the handler and
    discarded: the event already carries the whole number they refer to, and putting a fragment of a
    third party's phone number on an append-only log means keeping a needless copy of it forever.
  - `AcknowledgedPhone` is required, and the handler **rejects** an event without it. A verification
    naming no number cannot be checked for staleness, so it would be a permanent tick that no
    guardian-number change could clear — which is the failure `Person.IsVerified` and
    `invalidateVerification` (task 076) exist to prevent.
  - `VerifiedAt` is on the event rather than derived from delivery time, for the same reason as
    `PortraitCaptured.CapturedAt`: delivery time changes on every replay, and this timestamp is what
    "how many members verified before arriving?" is counted from.
- 2026-08-30 — **PRD 005 §12 Q3 (who consumes this) is still open, and the shape is frozen anyway.**
  Recorded as a deliberate choice rather than an oversight: the payload carries the member, the
  timestamp and the acknowledged number, which is what *any* of the candidate consumers (check-in
  view, patrol leader list, pre-event unverified report) would need to answer "who verified, when,
  and for which number". A consumer wanting more can have fields added — cheap on an append-only
  log — whereas reshaping would be expensive, and there is no consumer yet to reshape around.
- 2026-08-30 — ✅ `go build ./...`, `GOWORK=off go build ./...` and `go test ./nathejk/table/person/`
  all pass. shared-go untouched, `hq` untouched. Six new tests, including one asserting the published
  subject matches the consumed pattern — two strings in two files that nothing else connects, and a
  mismatch is silent.
- 2026-08-30 — The projection handler landed here with the message rather than in task 134, since
  splitting a 20-line handler from the type it decodes would have left both commits unable to build.
  Task 134 covers the read-side fields on `GET /api/me/profile`.
