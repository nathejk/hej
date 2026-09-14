# 228 — `POST /me/profile/skip` and the shared outcome publish

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

## Description

Today "spring over" happens only in the client, which is exactly the silence PRD 015 exists
to remove: the counter cannot tell a member who gave up from a member who never opened the
app. Add `POST /me/profile/skip` so the give-up is a recorded fact.

It publishes `NathejkMemberVerified{MemberID, Phone, VerifiedAt}` with an **empty**
`PhoneContact`. That is not a workaround for a missing message type — it is a true statement:
the member's own number *is* verified, because the PIN proved it, and the contact number is
not. Per PRD 015 §11, `omitempty` making this byte-identical to a login event is accepted:
both answer the only question a consumer asks ("is there a verified contact number yet?")
with "not yet".

The endpoint is shared by two callers: the explicit "spring over" tap, and exhaustion after
three failed attempts (task 227). Both produce the same outcome, so both go through the same
publish path.

Two behaviours that differ from the login-side publish (task 226) and must not be
copy-pasted from it:

- **The outcome publish always fires**, even when the member's own number was already
  recorded as verified. The once-per-number suppression is a login-side optimisation only;
  suppressing an outcome would put us back to silence for precisely the members this PRD is
  about.
- Outcomes must be **idempotent per member per check**: a double submit or a retry after a
  dropped connection must not read as two members' worth of signal.

A skip whose POST cannot reach the BFF (no signal after login, broker down) must still let the
member into the app — login is the only mandatory step (PRD 005 §6). The outcome is then lost,
which is accepted: check-in is the backstop, and queuing it would introduce the offline
layer's first queued mutation (PRD 009) for a low-stakes fact.

Full OpenAPI annotations are required — every endpoint in this repo has them (`.rules`).

Depends on tasks 222 and 223.

## Acceptance Criteria

- [ ] `POST /me/profile/skip` exists, is authenticated as the other `/me/profile` endpoints
      are, and has full OpenAPI annotations including what it records
- [ ] It publishes one event with `Phone` and `VerifiedAt` set and `PhoneContact` empty
- [ ] It publishes even when the member's own number was already recorded as verified — a
      test covers the second-login-then-skip sequence
- [ ] A double submit produces one outcome, not two
- [ ] Task 227's exhaustion path goes through the same publish, verified by a test that the
      two paths produce identical events
- [ ] The endpoint never blocks entry to the app: a failing publish does not leave the member
      stuck on the check

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
