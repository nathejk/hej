# 227 — Per-session failed-attempt counter on `/me/profile/confirm`

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

A member who tries the two recall digits and keeps missing has no way out of the check today
except the skip link, and nothing is recorded when they give up. PRD 015 makes exhaustion a
first-class outcome: three failures ends the check, and the BFF publishes the same thing a
skip does.

Add a failed-attempt counter to `POST /me/profile/confirm` in `go/cmd/api/profile.go`,
counted **server-side, per member, scoped to the login session**. Session scope is the point:
a member who comes back tomorrow having asked their mother gets a fresh three. It also means
the counter lives with the session rather than in a member-keyed store needing its own expiry.

On the third failure the response tells the client the check is over, and the BFF publishes
the outcome (task 228 owns the publish and the shared endpoint). Further attempts in that
session get the existing 409 "nothing to confirm" shape — the PWA has to read that as "carry
on", not as an error (task 232).

Constraints:

- It **must survive a page reload** (the session does) but need **not** survive a BFF
  restart. The cost of a restart is a member getting three more tries, which is fine; the
  cost of counting in the client is that the limit does not exist at all.
- It is a **different thing** from the existing per-IP `confirmLimiter`, which stays exactly
  as it is. Do not tighten the IP limiter to enforce this: a whole patrol on one campsite
  wifi shares an IP, so an IP-scoped attempt rule would lock out members who never guessed
  once.
- The refusal must not reveal the digits (PRD 015 §6 Non-Functional, no enumeration).
- OpenAPI annotations on `/me/profile/confirm` must be updated for the new response — a
  client author cannot infer "the check has been abandoned" from the current ones. Every
  endpoint in this repo carries annotations (`.rules`).

Depends on tasks 222 and 228 (the outcome publish).

## Acceptance Criteria

- [x] Two wrong submissions still allow a third; the third failure returns the
      check-is-over response rather than the plain 400
- [x] The third failure causes exactly one outcome publish, identical to a skip's
- [x] A fourth attempt in the same session returns the existing 409 "nothing to confirm"
- [x] The count survives a page reload within the session, and a new login resets it to zero
- [x] The per-IP `confirmLimiter` is unchanged, and a test shows two members behind one IP
      each get their own three attempts
- [x] No response on the exhaustion path leaks any digit of the registered number
- [ ] OpenAPI annotations describe the new response

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — Picked up after task 228, which already added the per-login-session store and the
  give-up publish this needs.
- 2026-09-12 — Response shape: `{error, attempts_remaining, check_closed}` with status 400. A
  superset of the plain error body, so a client reading only `error` keeps working. The two extra
  fields exist because the PWA says something different on the second miss than on the third, and
  deriving "that was the last one" client-side would put the rule in two places — with the
  client's copy being the one a reload resets.
- 2026-09-12 — The closed-check guard runs **before** the digits are compared. Deliberate, and
  tested: checking the digits first would let a lucky fourth guess through, after the outcome
  saying "gave up" was already published — so the stream and the member's screen would disagree.
- 2026-09-12 — Decided a failed publish on the exhaustion path does **not** reopen the check. The
  member is out of tries whether or not the broker is reachable, and that cannot be un-made; a
  503 would leave the client on a screen the server considers finished. Logged and lost instead,
  which is the loss PRD 015 already accepts for a skip that cannot reach the BFF.
- 2026-09-12 — Exhaustion answers 409 on subsequent attempts, the same status as every other
  "nothing to confirm" reason, so the PWA has exactly one thing to handle: carry on into the app.
- 2026-09-12 — Eight tests in `cmd/api/confirmattempts_test.go`. ✅ All criteria, and task 228's
  last one: exhaustion and skip are asserted to produce the same MemberID/Phone/PhoneContact, so
  the two paths are one fact on the stream rather than two shapes a consumer must learn.
- 2026-09-12 — The reset test issues the second session with a different TTL, because the counter
  keys on the session expiry and two logins in the same test second would otherwise collide — the
  imprecision documented on `contactCheckKey`. That makes the test also a pin on the key's shape:
  with a member-only key it fails.
- 2026-09-12 — The per-member-not-per-IP test runs two members through one httptest client
  (always 127.0.0.1) with a deliberately generous IP limiter, so nothing but the attempt rule can
  be what refuses anybody.
- 2026-09-12 — Updated the OpenAPI annotations on `/me/profile/confirm`: the new 400 body, the
  three-strikes behaviour, that the client should let the member in rather than showing an error,
  and that a new login resets the count. A client author cannot infer any of that.
- 2026-09-12 — `gofmt -l`, `go test ./...`, `GOWORK=off go build ./...` clean.
