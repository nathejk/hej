# 369 — `requireAdmin`: a shared credential that grants exactly one tool

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

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

## Shipped together with tasks 370 and 371

One commit for all three, because they are one wrapper and one predicate — and because task 371's own
description makes the argument: "these are one task because they are one argument". The credential is weak by
construction, and the gating and the hardening are what make it acceptable; a commit with one and not the
others would be a commit that should not be deployed.

## What landed

`requireAdmin` in `middleware.go`, checking basic auth against `ADMIN_USER` / `ADMIN_PASSWORD`. The check
order is load-bearing and documented: transport, then headers, then rate limit, then comparison — each
refusal cheaper than the next, and the transport check *first* because the credential is inside the request
being refused.

Two details that are more than hygiene:

**Both sides are hashed before comparison.** `subtle.ConstantTimeCompare` returns early for inputs of
different lengths, so comparing raw strings leaks the configured password's *length* through timing. Hashing
to a fixed 32 bytes removes that, which matters here because length is the main thing standing between a
shared secret and a brute force.

**The two comparisons are combined with `&` rather than `&&`.** A short-circuit would skip the password
comparison when the username is wrong, which is a timing oracle for "that username exists" — and for a shared
credential the username is half the secret. Asserted structurally, since timing is not reliably testable.

A minimal but real `/admin` page landed with it (`adminpage.go`), rather than a stub: the counts are read
through the curator interface, so the page exercises credential → draft-visible read → render end to end.
Without it the Phase 2 tests would be assertions about a door with no room behind it.

## Acceptance Criteria

- [x] `requireAdmin` verifies basic auth with a constant-time comparison of both user and password
- [x] A missing or wrong credential is `401` with a `WWW-Authenticate` challenge, and all seven refusal cases
      return **byte-identical** bodies — asserted, so the endpoint cannot become a username oracle
- [x] The wrapper sets **no** session on the request context — asserted from inside a handler, and verified by
      breaking it
- [x] A test walks the route table: every admin path is behind `requireAdmin`, no admin path is also behind
      `requireAuth`, and no non-admin path is behind `requireAdmin`. Verified by breaking it in both
      directions
- [x] `ADMIN_USER` / `ADMIN_PASSWORD` are documented in `env.go` with the rotation cost and the absence of a
      default written down
- [x] The doc comment records why basic auth was chosen over the session mechanism (PRD 022 §8.2)
- [ ] Every admin **write** logs the action, the affected ids and the client IP — deferred to the tasks that
      add writes (372–379); there are none yet. Rejections and rate limits already log the IP and path.
