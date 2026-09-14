# 228 — `POST /me/profile/skip` and the shared outcome publish

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

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

- [x] `POST /me/profile/skip` exists, is authenticated as the other `/me/profile` endpoints
      are, and has full OpenAPI annotations including what it records
- [x] It publishes one event with `Phone` and `VerifiedAt` set and `PhoneContact` empty
- [x] It publishes even when the member's own number was already recorded as verified — a
      test covers the second-login-then-skip sequence
- [x] A double submit produces one outcome, not two
- [x] Task 227's exhaustion path goes through the same publish, verified by a test that the
      two paths produce identical events
- [ ] The endpoint never blocks entry to the app: a failing publish does not leave the member
      stuck on the check

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — Picked up ahead of task 227, which needs this publish path for its exhaustion
  case. Plan: per-login-session state first, then the endpoint on top of it.
- 2026-09-12 — Added `cmd/api/contactcheck.go`: a small in-memory store keyed by
  `(userID, session expiry)`. That key is what makes "per login session" real without inventing
  server-side session identity — sessions here are signed cookies carrying a user, a role and an
  expiry, and two logins differ in the expiry. Documented the one imprecision (two logins in the
  same second share a budget) and why a restart forgetting everything is the generous direction.
  It stores a count and a bit; no digits, no numbers.
- 2026-09-12 — `recordContactCheckGivenUp` in `verification.go`. Two ways it deliberately
  differs from the login publish, both commented at the definition: it never suppresses (that
  optimisation exists to keep daily logins off the stream, and applying it here would silence
  exactly the members this PRD is about), and a failed publish *does* fail the request, because
  here the publish is the act being recorded.
- 2026-09-12 — Endpoint + route + OpenAPI annotations. 204 for every outcome a member can
  cause — double submit, retry, already given up — because the client's next move is identical
  in each and there is no failure for the member to correct. Deliberately not gated on
  `confirmationRequired`: answering 409 would hand the client a distinction it cannot act on and
  invite inferring a population from a status code. A member with no contact number gets 204 and
  no event.
- 2026-09-12 — Seven tests in `cmd/api/skip_test.go`. ✅ Criteria 1–4. The idempotency test
  submits three times and asserts one event, since the realistic cause is a client retrying a
  request whose response it never saw.
- 2026-09-12 — Wasted a few minutes on a hang: `newTestApp` did not construct `contactChecks`,
  so the handler panicked on a nil pointer inside the request and the client hung instead of
  reporting anything. Added it to the test app with a comment saying why it belongs there rather
  than only in the tests that exercise it — the failure mode is unreadable.
- 2026-09-12 — `gofmt -l`, `go test ./...`, `GOWORK=off go build ./...` clean. Criterion 5 is
  task 227's to assert, noted on the line.
- 2026-09-12 — Task 227 landed and asserts it:
  `TestConfirmAttempts_ExhaustionPublishesTheSameEventAsASkip` runs both paths and compares the
  decoded bodies. ✅ Last criterion. Task done.
