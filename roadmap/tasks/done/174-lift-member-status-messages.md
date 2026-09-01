# 174 — Lift member-status transition messages to `shared-go`

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed) — `hej` side; the lift itself by the maintainer
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

Follow-up from task 150's decision. PRD 007 needs to know when a member leaves the race
(`reunited` / `released`) so the contacts pane can mark them and purge their number.

The **vocabulary** was already shared (`shared-go/types/member.go`). What was not shared was the
**message types for the transitions**, which lived in `hq`'s `spejderstatus` package.

## What was lifted, and by whom

The maintainer lifted them on 2026-09-01: `hq/go/nathejk/table/spejderstatus/messages.go` →
**`shared-go/messages/member.go`**, renamed to the `Nathejk*` convention of that package.

Eight events, each resolving to exactly one `types.MemberStatus` through the
`NathejkMemberEvent` interface:

| Event | Subject suffix | Status |
|---|---|---|
| `NathejkMemberWithdrawalRequested` | `withdrawal.requested` | `waiting` |
| `NathejkMemberWithdrawalCancelled` | `withdrawal.cancelled` | `racing` |
| `NathejkMemberStatusOverridden` | `status.overridden` | `To` |
| `NathejkMemberTeamMoved` | `team.moved` | `racing` |
| `NathejkMemberPickupAccepted` | `pickup.accepted` | `transit` |
| `NathejkMemberShelterAccepted` | `shelter.accepted` | `sheltered` |
| `NathejkMemberShelterPlaced` | `shelter.placed` | `sheltered` |
| `NathejkMemberHandoverCompleted` | `handover.completed` | `To` |

Worth recording: `hq`'s package was *written* to be lifted — its doc comment says so, and
`lift_test.go` parses every file in it to fail if anything imports `nathejk.dk/...`. That is why
this was a move rather than a rewrite. `hq`'s own task **083** covers lifting the whole
`spejderstatus` *projection* later; only the events were needed here, and hq 083's own
prerequisite list names the member events settling in shared-go as a precondition, so they were
always meant to move first.

## The `hej` side (this task's work)

`go/nathejk/table/person/consumer.go` + `memberstatus_test.go`.

**All eight subjects consumed, not just the two endings.** PRD 007's patrol lookup shows the full
status, and `waiting`/`transit`/`sheltered` are exactly what a samarit needs *before* setting off —
somebody else already has the member. `pickup.accepted` is published by nobody yet (it belongs to
the car interface); subscribing now is what makes this ready rather than something to revisit.

**One handler, driven by the event's own `Status()`.** `memberStatusEventFor` maps a subject to an
empty body of the right type; the handler decodes, asks the body what status it means, and writes
it. So this projection never decides what a transition means, which is the line task 150 drew:
`hej` caches one shared notion of status, it does not grow a second one. A transition added
upstream is one map entry and one subject here.

**The ordering trap, avoided deliberately.** These subjects carry an extra event segment, so they
are matched *before* the four-part `spejder.*.updated` / `.deleted` patterns — hq's projector
carries a warning that the reverse ordering has bitten this codebase before. There is a test
asserting a withdrawal is not projected as a member update or a deletion.

**Member id from the subject, not the body.** Authoritative and present on all eight, whereas the
bodies name it differently. Same extra-segment shape as the arm-number subject, so the id is
`parts[3]` rather than what `subjectEntityID` assumes — the existing precedent, reused.

**Unknown or empty statuses are dropped, not stored.** `Valid()` is shared-go's definition, so an
unrecognised value can only mean upstream is ahead of this deployment; the member's next event
corrects it. Storing it would put a value on the wire the client would then have to render.

**Rewrote the `handleTeamStarted` doc comment** rather than deleting the part that is now false. It
used to say this projection deliberately consumes *one* transition; it now records why that
changed, and keeps the warning that still stands — a second *notion* of status is the thing to
avoid, and storing `types.MemberStatus` verbatim from the same events is a cache, not a second
notion.

**Updated `stillInRace()` in `cmd/api/contacts.go`**, which claimed the field reads `true` for
everyone until this task landed. It now reads real statuses, so PRD 007's withdrawal marking and
phone purge are live rather than inert.

## Gap found and split out

`team.moved` carries `FromTeamID`/`ToTeamID` and this handler ignores both, writing only the
status — so a moved member's team goes stale and PRD 007's patrol lookup would show them under
their old patrol. Split out as **task 178** with the reasoning, and pinned by
`TestTeamMovedDoesNotChangeTheTeam` so today's deliberate gap cannot be mistaken for an accident.

## Acceptance Criteria

- [x] Member-status transition message types defined in `shared-go`, using
      `types.MemberStatus` — `messages/member.go`, by the maintainer.
- [x] `hq` publishes (or continues to publish) those types, with no duplicate definition
      left behind — maintainer's side; hq's task 083 covers the projection itself.
- [x] `hej`'s person projection consumes the transitions and stores the status verbatim
      — no locally-invented strings, no re-derivation.
- [x] `person.memberStatus` can hold any valid `types.MemberStatus`, not just `racing`.
- [x] The `handleTeamStarted` comment is updated: it stated this projection deliberately
      consumes only one transition, which is no longer true.
- [x] Replay-safe: a full replay produces the same statuses as the live stream —
      `TestMemberStatusIsIdempotent`; every write is a plain `UPDATE`.

## Progress Log

- 2026-08-31 — Task created as a follow-up from task 150's decision.
- 2026-09-01 11:10 — Maintainer reported the lift into `shared-go/messages/member.go`. Verified all
  eight events and the `NathejkMemberEvent` interface are present and resolvable from `hej` (local
  path replace, so no pin bump needed).
- 2026-09-01 11:20 — Consumed all eight subjects, matched ahead of the four-part spejder patterns,
  with a test for the ordering trap hq warned about.
- 2026-09-01 11:30 — Found `team.moved`'s team ids are ignored; split out as task 178 rather than
  widening this change, and pinned the current gap with a test.
- 2026-09-01 11:35 — ✅ All criteria met. `go build`, `go vet`, `go test ./...` clean. Task 175 (the
  "left the race" predicate in shared-go) remains open and is still the interim in
  `cmd/api/contacts.go`.
