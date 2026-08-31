# 174 — Lift member-status transition messages to `shared-go`

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

Follow-up from task 150's decision. PRD 007 needs to know when a member leaves the race
(`reunited` / `released`) so the contacts pane can mark them and purge their number.

The **vocabulary** is already shared: `shared-go/types/member.go` defines the full
lifecycle with docs and helpers. What is not shared is the **message types for the
transitions** — per the comment above `handleTeamStarted` in
`go/nathejk/table/person/consumer.go`, those are "defined in that repo", i.e. `hq`.
`shared-go/tables/spejder/consumer.go` consumes only `spejder.*.updated`,
`spejder.*.deleted` and `patrulje.*.started`.

So the transition messages need lifting to `shared-go`, exactly as task 147 lifted the
verification messages. Then `hej`'s person projection can consume them and store
`types.MemberStatus` verbatim.

**Scope discipline:** lift the message *types* (and, if it lives with them, the
transition validation). Do **not** lift or duplicate `hq`'s full `spejderstatus`
projection into `hej` — task 150's decision is that `hej` caches the value, and lifecycle
*rules* stay in `shared-go`.

This is work in a sibling repo (`shared-go`, and possibly `hq`), so it cannot be done
from the `hej` workspace alone.

## Acceptance Criteria

- [ ] Member-status transition message types defined in `shared-go`, using
      `types.MemberStatus`.
- [ ] `hq` publishes (or continues to publish) those types, with no duplicate definition
      left behind.
- [ ] `hej`'s person projection consumes the transitions and stores the status verbatim
      — no locally-invented strings, no re-derivation.
- [ ] `person.memberStatus` can hold any valid `types.MemberStatus`, not just `racing`.
- [ ] The `handleTeamStarted` comment is updated: it currently states this projection
      deliberately consumes only one transition, which will no longer be true.
- [ ] Replay-safe: a full replay produces the same statuses as the live stream.

## Progress Log

- 2026-08-31 — Task created as a follow-up from task 150's decision.
