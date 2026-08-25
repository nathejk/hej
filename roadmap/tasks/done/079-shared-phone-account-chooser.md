# 079 — Post-verification account chooser for shared phone numbers

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

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

- [x] `verify` returns candidates + a signed choice token for a shared number
- [x] `choose` exchanges the token for a session, and only for a listed candidate
- [x] Token is short-lived, signed, and bound to the verified phone number
- [x] The chooser is unreachable without a successful PIN verification
- [x] Candidate payload carries no more than a name and team
- [x] An unknown number still behaves identically to a known one (anti-enumeration)
- [x] Frontend login step renders the chooser
- [x] OpenAPI annotations on both endpoints
- [x] Tests: single owner (unchanged), two owners, forged/expired token, token
      reused for a candidate not on the list

## Progress Log

- 2026-08-25 — Task created as a follow-up from task 071's decision.
- 2026-08-25 — Picked up. New `internal/choice` package for the token, reusing the
  session manager's HMAC construction rather than inventing crypto.
- 2026-08-25 — Deliberately **the same signing secret** as sessions. Both are
  server-side keys with the same blast radius, and a second secret to configure is a
  second secret to forget to set in production — which fails open, not closed.
- 2026-08-25 — The token carries **only** the verified phone number and an expiry. No
  user id, by design: it authorises "pick one of this number's owners", not "be this
  user". A test pins that, so nobody adds an id for convenience later.
- 2026-08-25 — Three properties make `/auth/choose` safe to expose, and all three are
  load-bearing, so they are listed in the handler doc:
  1. the token is only minted after a successful PIN verification;
  2. it is bound to that phone number, so it cannot be redeemed against another's
     owners;
  3. the chosen user is **re-checked against the directory** at redemption, not trusted
     from the request. Without (3), a verified PIN for any number would become a session
     as any user in the system.
- 2026-08-25 — Security detail: expiry is checked **after** the signature, so a forged
  token always reports `ErrInvalid` and never `ErrExpired`. Reporting expiry would
  confirm to a forger that their signature had been accepted. Covered by a test.
- 2026-08-25 — Shared number returns **HTTP 200**, not an error: verification did
  succeed. The client branches on the presence of `choice_token`, so this is a different
  shape of success rather than a failure to interpret. Documented in the OpenAPI
  description because it is genuinely surprising.
- 2026-08-25 — Privacy: the candidate payload is `user_id`, **first name** and team.
  Added `Name` to `users.User` for this and nothing else. First name only — "Freja" is
  enough for a sibling to recognise themselves, while "Freja Mikkelsen" hands the phone's
  holder a fuller identifier for someone who is not them. A test asserts the response
  carries no address, phone, birthday, email or role.
- 2026-08-25 — Found while wiring the frontend: the PIN form was `v-else`, so adding a
  third step would have made the chooser unreachable while the PIN form rendered over it.
  Changed to an explicit `v-else-if`.
- 2026-08-25 — The choice token is held **in memory only** in the store, never
  persisted. It lives ~2 minutes and exists to bridge two requests; persisting it would
  give it a lifetime it should not have. A 401 on choose is treated as "took too long",
  returning the user to the phone step with a clear message rather than a dead end.
- 2026-08-25 — ✅ All criteria complete. 8 token tests + 7 handler tests, and the three
  pre-existing verify tests still pass unchanged — which was the point of keeping the
  single-owner path identical.

### Verified against the running stack

- 2026-08-25 — `vue-tsc --noEmit` clean (exit 0) in the `ui` container. That also
  retroactively closes **task 067's** unverified caveat: its role-enum change had never
  been type-checked, because there was no Node runtime at the time.
- 2026-08-25 — End-to-end against the live API:
  - shared number → 200 with a token and two candidates, **no session cookie**, names
    rendered as `"Freja"` / `"Villads"` (first names only, as designed)
  - choose a legitimate owner → 200 + `Set-Cookie: hej_session=…`
  - **valid token + a user who owns a different number → 401, no cookie** (the attack
    this endpoint exists to resist)
  - **forged signature → 401, no cookie**
- 2026-08-25 — Limitation worth stating: the live API still reads the **mock**
  directory, so this was exercised against the seeded sibling pair, not the 213 real
  shared numbers. One of those is `+4521340605` (2 owners) in the projection right now.
  Task 077's swap is what makes the real ones reachable — and this task was the blocker
  on 077, so that is now unblocked.
- 2026-08-25 — Moving to done.
