# 222 — Reshape `NathejkMemberVerified` to carry two optional phone numbers

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 015 turns the first-login guardian check into a check-in fast track, and the message
that carries the outcome does not currently fit: `messages.NathejkMemberVerified` in the
sibling `shared-go` checkout (`../../shared-go`, wired in via `go/go.work`) is shaped
around "the member confirmed the register's number", carrying `PhoneParentAcknowledged`,
`PhoneParentRegistered` and `Year`.

Reshape it to:

```go
type NathejkMemberVerified struct {
	MemberID     types.MemberID    `json:"memberId"`
	Phone        types.PhoneNumber `json:"phone,omitempty"`
	PhoneContact types.PhoneNumber `json:"phoneContact,omitempty"`
	VerifiedAt   time.Time         `json:"verifiedAt"`
}
```

`Phone` is the member's own number, proven by the SMS PIN they typed at login.
`PhoneContact` is the contact number the member acknowledged, using the register's own
field name (`NathejkScoutUpdated.PhoneContact`) rather than the projection's `phoneParent`.
Either may be absent, which is exactly what lets the two verifications be published
independently: the login event carries only `Phone`, the guardian check carries both.

`PhoneParentRegistered` is dropped on purpose (PRD 015 §4): the only question a consumer
asks is whether a verified contact number exists yet, not whether the register moved or the
member corrected us. `Year` is dropped because it is the second subject token and the
consumer already receives it separately.

`omitempty` on both fields is a **deliberate** decision, not an oversight, and the doc
comment must say so: it means a skip event serialises byte-identically to a login event.
That costs nothing, because both answer the one question ("is there a verified contact
number for this member yet?") with "not yet". Without that note the next reader will assume
the transmitted-empty reading and "fix" it.

The doc comment has to be **rewritten**, not trimmed — it documents a shipped contract that
other consumers read, and the new rationale (fast track, PIN-proven `Phone`, acknowledged
`PhoneContact`, either may be absent) is not a subset of the old one.

**This task blocks every other Go task in PRD 015** (223, 224, 225, 226, 227, 228, 230,
231). Note that CI and production build with `GOWORK=off` against the version pinned in
`go/go.mod`, so shipping this includes cutting a `shared-go` version and bumping that line
— a change that only builds with the workspace active is a broken build.

## Acceptance Criteria

- [ ] `messages.NathejkMemberVerified` has exactly the four fields above, and
      `PhoneParentAcknowledged`, `PhoneParentRegistered` and `Year` are gone
- [ ] The doc comment states the new contract: check-in fast track, `Phone` proven by SMS
      PIN, `PhoneContact` the acknowledged contact number, either may be absent
- [ ] The doc comment records that `omitempty` on `PhoneContact` is deliberate and why a
      skip event being byte-identical to a login event is acceptable
- [ ] `shared-go` builds and its own tests pass after the reshape
- [ ] `hej/go` builds with `GOWORK=off` against a bumped `go.mod` version, not only with the
      workspace active
- [ ] No remaining reference to the dropped fields anywhere in `shared-go` or `hej/go`

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
