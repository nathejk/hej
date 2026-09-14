# 226 — Publish own-phone-verified on a successful PIN login

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

To reach the app at all a member receives a PIN by SMS on the number we hold for them and
types it back. Their own number is therefore verified, by a real challenge-response, at no
cost to anybody — and today we throw that away (PRD 015 §2). Publishing it is another
question the check-in counter does not have to ask.

On a successful PIN login (`go/cmd/api/auth.go`), publish
`NathejkMemberVerified{MemberID, Phone, VerifiedAt}` with no `PhoneContact` at all.

Scope, per PRD 015 §6: **spejder and bandits only**. Crew and gøgler have their own numbers
verified through another route, so an event for them would restate a known fact. Bandits have
no contact number (`phoneParent == nil`) and are never asked for one, but their own number is
still worth recording — which is why the `spejder` subject token is a token and not a
population (task 223).

Frequency: once per member **per verified number**, not once per login. The fact does not
change between logins and login volume on the stream would say the same thing repeatedly.
Suppress the publish **only** while the number already recorded as verified is the number the
PIN just proved — so if a member's number changes and a later login proves a different one,
the later verification supersedes the earlier (last verification wins).

Two constraints that are easy to get wrong:

- **This must never fail the login.** It is a by-product; a broker outage must not stop
  people getting into the app. Log the failure and carry on. This is the explicit exception
  to the rule in `verification.go` that a failed publish fails the request.
- The suppression check reads state before publishing, which makes it read-then-write and
  therefore racy under two simultaneous logins. **Accepted** — the events are idempotent in
  meaning (same member, same number, same claim) and the projection takes the latest. Do not
  add locking.

Depends on tasks 222 and 223.

## Acceptance Criteria

- [x] A first successful PIN login by a spejder publishes one event carrying `Phone` and
      `VerifiedAt` and no `PhoneContact` field
- [x] A bandit login publishes the same; a crew or gøgler login publishes nothing
- [x] A second login with the same number publishes nothing; a login proving a *different*
      number publishes again
- [x] A publish failure is logged and the login still succeeds — covered by a test with a
      failing publisher, asserting the 2xx and the issued session
- [x] No locking or serialisation was added around the read-then-publish sequence

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — Picked up. Plan: read the own-phone columns into `Person` so the publish can be
  suppressed, then a `recordOwnPhoneVerified` helper in `verification.go` called from both
  places that issue a session (`verifyPinHandler` and `chooseHandler`) — log-and-continue on
  every failure.
- 2026-09-12 — Added `PhoneVerifiedAt`/`VerifiedPhone` to `Person` and the SELECT list — the
  columns task 224 writes, now read for exactly one purpose. Said so on the fields, including
  that they are deliberately not part of `IsVerified`.
- 2026-09-12 — Put the suppression rule in `Person.PhoneVerifiedIs(normalized)` rather than
  comparing at the call site, because "once per member per year" and "last verification wins"
  are one rule and would otherwise become two. Both halves are load-bearing: the timestamp
  alone republishes forever, and ignoring the number means a member who changes phones never
  gets the new one recorded.
- 2026-09-12 — `recordOwnPhoneVerified` in `verification.go`, called from **both** places that
  issue a session: `verifyPinHandler`'s single match and `chooseHandler`'s shared-number path.
  The second is easy to miss and is exactly where a shared number resolves to a person — two
  siblings on one phone legitimately both record it as verified.
- 2026-09-12 — Every failure path returns silently or logs at Warn: no person row, an
  unbuildable subject, a failed publish. Nothing here can fail a login, which is the explicit
  exception to `storeVerification`'s "a failed publish fails the request" — and the event is
  not lost, because nothing was recorded to suppress the next login's attempt.
- 2026-09-12 — Eight tests in `cmd/api/ownphone_test.go`, all driving the real `/auth/verify`
  endpoint rather than the helper: a unit test of the helper would keep passing if the call
  site were deleted, and the property is "logging in records this". Covers spejder, bandit,
  three non-publishing roles, suppression, a newly proven number, no broker, a refusing broker,
  and no projection row. ✅ All criteria.
- 2026-09-12 — Had to relax one assertion in `photo_test.go`, which required *exactly one*
  person lookup per request; logging in now performs one of its own. Rewrote it to check that
  every lookup is keyed by the configured year and the session's user, which is the property
  it was actually defending — a count would keep breaking on unrelated legitimate reads.
- 2026-09-12 — `gofmt -l`, `go test ./...` and `GOWORK=off go build ./...` clean.
