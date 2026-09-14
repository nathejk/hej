# 227 — Per-session failed-attempt counter on `/me/profile/confirm`

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] Two wrong submissions still allow a third; the third failure returns the
      check-is-over response rather than the plain 400
- [ ] The third failure causes exactly one outcome publish, identical to a skip's
- [ ] A fourth attempt in the same session returns the existing 409 "nothing to confirm"
- [ ] The count survives a page reload within the session, and a new login resets it to zero
- [ ] The per-IP `confirmLimiter` is unchanged, and a test shows two members behind one IP
      each get their own three attempts
- [ ] No response on the exhaustion path leaks any digit of the registered number
- [ ] OpenAPI annotations describe the new response

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
