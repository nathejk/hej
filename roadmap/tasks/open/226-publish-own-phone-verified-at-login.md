# 226 — Publish own-phone-verified on a successful PIN login

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A first successful PIN login by a spejder publishes one event carrying `Phone` and
      `VerifiedAt` and no `PhoneContact` field
- [ ] A bandit login publishes the same; a crew or gøgler login publishes nothing
- [ ] A second login with the same number publishes nothing; a login proving a *different*
      number publishes again
- [ ] A publish failure is logged and the login still succeeds — covered by a test with a
      failing publisher, asserting the 2xx and the issued session
- [ ] No locking or serialisation was added around the read-then-publish sequence

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
