# 222 — Reshape `NathejkMemberVerified` to carry two optional phone numbers

**Status:** doing
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
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

- [x] `messages.NathejkMemberVerified` has exactly the four fields above, and
      `PhoneParentAcknowledged`, `PhoneParentRegistered` and `Year` are gone
- [x] The doc comment states the new contract: check-in fast track, `Phone` proven by SMS
      PIN, `PhoneContact` the acknowledged contact number, either may be absent
- [x] The doc comment records that `omitempty` on `PhoneContact` is deliberate and why a
      skip event being byte-identical to a login event is acceptable
- [x] `shared-go` builds and its own tests pass after the reshape
- [ ] `hej/go` builds with `GOWORK=off` against a bumped `go.mod` version, not only with the
      workspace active — **blocked on pushing `shared-go` 51bff56**, see log
- [x] No remaining reference to the dropped fields anywhere in `shared-go` or `hej/go`

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — Picked up. Plan: reshape the struct and rewrite its doc in the sibling
  shared-go checkout, then fix the two call sites in hej (`verified.go`'s projection
  handler and `cmd/api/verification.go`'s publish) so the workspace build is green before
  cutting a version.
- 2026-09-12 — Struct reshaped and doc rewritten in `../shared-go/messages/member.go`.
  `gofmt`, `go build ./...` and `go test ./messages/...` clean there. Committed as
  shared-go 51bff56.
- 2026-09-12 — Kept `storeVerification`'s `registeredPhone` parameter, unused, rather than
  changing every caller in the same commit: the callers get reworked by tasks 227/228/231
  anyway, and a signature change threaded through them here would bury the contract change
  it is meant to demonstrate. Marked `_ = registeredPhone` with the reason.
- 2026-09-12 — `handleMemberVerified` still refuses an event with no contact number. That is
  deliberate for this task: an own-phone-only event has nowhere to be stored until task 224
  adds the second column, and no publisher sends one yet. The comment says so, and task 224
  is where the refusal is lifted.
- 2026-09-12 — `verifiedAgainstPhone` is now written as NULL, because the event no longer
  carries the register's value and reading the current `phoneParent` here would be wrong on
  replay. Task 225 removes the column and its reader.
- 2026-09-12 — Tests updated in `nathejk/table/person/verified_test.go` and
  `cmd/api/confirm_test.go`. The correction test no longer asserts two different numbers (it
  cannot); instead it asserts the *encoded body* carries no `phoneParentRegistered`, so
  reintroducing the field has to be a decision. ✅ Four of six criteria.
- 2026-09-12 — `go vet ./...`, `gofmt -l` and `go test ./...` all clean with the workspace
  active. **`GOWORK=off go build` fails, as expected**: it resolves the version pinned in
  `go.mod`, which predates the reshape. That criterion needs shared-go 51bff56 pushed and
  the `go.mod` line bumped — pushing another repo is not mine to do unasked, so it is left
  open and flagged rather than quietly ticked. Everything else in PRD 015 builds and tests
  against the workspace in the meantime.
