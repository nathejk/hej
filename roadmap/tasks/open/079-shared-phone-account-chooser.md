# 079 — Post-verification account chooser for shared phone numbers

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

Follow-up from task 071, which decided the policy: when a phone number belongs to
several people (siblings sharing a phone, or a guardian's number entered as the
scout's own), the user **verifies the PIN first and then chooses who they are**.

Task 071 implemented the directory half — `LookupAll` returns every owner, and
`Lookup` returns not-found for a shared number so nobody can be silently logged in
as their sibling. This task implements the flow.

**Until this ships, a shared number cannot log in at all.** That is a deliberate
fail-safe, not an oversight, but it means this task must land **before or with**
task 077 (the swap to the real projection) — otherwise real colliding users are
locked out of production.

### Shape

- `POST /api/auth/verify` on a shared number returns the candidate list plus a
  short-lived, signed **choice token** instead of a session. HTTP 200 with a
  distinct response shape, not an error.
- `POST /api/auth/choose` exchanges `{token, user_id}` for a session.
- The token must be single-use-ish, short-lived (a minute or two), bound to the
  verified phone number, and signed like the session cookie — so the chooser cannot
  be reached without having passed PIN verification.
- The candidate list is the minimum that lets a person recognise themselves:
  a first name and perhaps a team. **Not** addresses, not full profiles, not
  birthdays.
- Frontend: a step in the login flow (PRD 005 `WelcomeStepLogin`), not a separate
  route.

### Privacy note

This deliberately shows one person the (first) name of everyone else on that
number. That is defensible because whoever holds the phone already shares a
household with them — but it is a disclosure, so keep the payload minimal and do
not let the endpoint be reachable without a verified PIN.

## Acceptance Criteria

- [ ] `verify` returns candidates + a signed choice token for a shared number
- [ ] `choose` exchanges the token for a session, and only for a listed candidate
- [ ] Token is short-lived, signed, and bound to the verified phone number
- [ ] The chooser is unreachable without a successful PIN verification
- [ ] Candidate payload carries no more than a name and team
- [ ] An unknown number still behaves identically to a known one (anti-enumeration)
- [ ] Frontend login step renders the chooser
- [ ] OpenAPI annotations on both endpoints
- [ ] Tests: single owner (unchanged), two owners, forged/expired token, token
      reused for a candidate not on the list

## Progress Log

- 2026-08-25 — Task created as a follow-up from task 071's decision.
