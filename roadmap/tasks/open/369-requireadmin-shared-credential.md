# 369 — `requireAdmin`: a shared credential that grants exactly one tool

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

Add `app.requireAdmin(next)` beside `app.requireAuth` in `go/cmd/api/middleware.go`, checking HTTP
basic auth against `ADMIN_USER` / `ADMIN_PASSWORD` read through `go/cmd/api/env.go`'s `envStr`.

This must be recorded as a **deliberate exception**, because it is genuinely new to the service.
PRD 022 §8.2: today there is no inbound credential of any kind in `go/` — every authenticated route is
`requireAuth` → an HMAC-signed session cookie from an SMS PIN → a per-request lookup in the `person`
projection for authorization. No bearer token, no API key, no admin role. The single
`Authorization: Basic` in the tree is outbound, to the SMS provider.

The existing mechanism was considered and does not fit. It requires the curator to be a person in this
year's `person` projection, with a phone number we hold, reachable by SMS, and — because the section
lookup via `isGlimtModerator` is how authorization works — assigned to the right Team section upstream.
Our photographers are frequently none of those things: volunteers with a camera, sometimes not
registered as personnel at all, and the tool has to work for them on the Tuesday after the event when
the SMS pipeline is the last thing anybody wants in the loop.

What it costs, stated plainly rather than discovered: **no attribution** (so the projections carry no
curator field and must not pretend to), **a shared secret that will be pasted into a chat message**,
rotation only by config change and redeploy, and no way to revoke one person without rotating for
everybody.

One structural mitigation is load-bearing: the credential grants **exactly this tool**. It is not a
session, it **must not populate the request context** via `contextSetSession`, and it must not become a
second way to reach any existing authenticated endpoint. The comparison is constant-time
(`crypto/subtle`), and a wrong credential is a bare `401` with the browser's own dialog — no custom
login page, no lockout, deliberately no "forgot password".

## Acceptance Criteria

- [ ] `requireAdmin` verifies basic auth with a constant-time comparison of both user and password
- [ ] A missing or wrong credential is `401` with a `WWW-Authenticate` challenge and no body detail
      that distinguishes "wrong user" from "wrong password"
- [ ] The wrapper sets **no** session on the request context — asserted by a test, so a handler behind
      it cannot call `contextGetSession` and get a caller
- [ ] A test asserts no route outside the admin surface is wrapped in `requireAdmin`, and no admin
      route is wrapped in `requireAuth`
- [ ] `ADMIN_USER` / `ADMIN_PASSWORD` are documented in `env.go` with the rotation cost written down
- [ ] The doc comment records why basic auth was chosen over the session mechanism (PRD 022 §8.2)
- [ ] Every admin write logs the action, the affected ids and the client IP — with a shared credential
      the log is the only audit trail there is
