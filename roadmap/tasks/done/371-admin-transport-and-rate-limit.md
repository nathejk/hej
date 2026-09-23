# 371 — HTTPS-only, `no-store`, `noindex`, and a rate limit on the guess

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

The hardening PRD 022 §6 Non-Functional and §8.2 require of every admin response, all of it following
from the fact that the credential is sent on **every** request rather than exchanged for a session.

- **HTTPS-only.** In production Traefik terminates TLS, but the handler additionally refuses to serve
  over plain HTTP unless `ENV=development`. A basic-auth credential over HTTP is the password in
  cleartext on every single request, including the ones the user did not think of as sensitive.
- **`X-Robots-Tag: noindex, nofollow` and `Cache-Control: no-store`** on every admin response,
  including the JSON ones and the media bytes. An admin page in a search index is an invitation, and a
  cached contact sheet on a shared laptop is a leak.
- **Rate limiting on the credential check, by client IP**, so a shared password is not brute-forceable
  at speed. Note the existing per-user glimt limiters do not apply here — there is no user (PRD 022
  §8.9) — so this is a new limiter keyed on IP, sitting in front of the comparison rather than behind
  it.

These are cheap individually and are one task because they are one argument: the credential is weak by
construction (§8.2 — shared, pasted into chat, rotated only by redeploy), and the mitigations are what
make that acceptable. Skipping any one of them removes a leg from a decision that was approved on the
strength of all of them.

## What landed

All three mitigations inside `requireAdmin`, so a new admin handler gets them by being behind the wrapper
rather than by remembering. Shipped with tasks 369 and 370 — this task's own description makes the case for
that ("one task because they are one argument").

**Transport.** `adminTransportOK` refuses plain HTTP outside development, answering **421 Misdirected
Request** rather than redirecting to https. A redirect would be friendlier and is wrong: the credential is
already in the cleartext request, so the damage is done, and inviting a retry makes the same mistake look
successful. `X-Forwarded-Proto` is consulted because Traefik terminates TLS and `r.TLS` is nil in production —
with a note that the header is trustworthy only *because* the sole route to this service is our own proxy.

**Headers.** `setAdminHeaders` is called on the way in, before any outcome, so `no-store` and `noindex` are on
the 401s, the 429s and the 421s too — not just the successes. A cached admin response on a shared laptop is a
leak whatever its status code.

**Rate limit.** A new IP-keyed limiter, 30 attempts per hour, in front of the comparison. Every attempt counts,
not only failures: exempting successes would let an attacker who guessed correctly continue unthrottled, and
counting only failures still requires doing the comparison to find out which it was. Hourly rather than
per-minute because this guards a *secret* rather than a resource — sustained slow guessing is the attack, and a
per-minute window forgives it every sixty seconds.

The ceiling is a constant rather than configuration, unlike the glimt limits: making it tunable would mean the
number protecting a shared password can be raised by whoever is debugging a lockout at the time.

## Verified live, not just in tests

Against the dev stack through Traefik: 34 wrong-password requests produced `401`×26 then `429`, the rejections
were logged with the client IP, and the **correct** credential was throttled too — the documented tradeoff,
confirmed in production-like conditions rather than asserted. Response headers checked on the wire: `401` with
`cache-control: no-store`, `x-robots-tag: noindex, nofollow` and the `WWW-Authenticate` challenge.

## Acceptance Criteria

- [x] A plain-HTTP request to any admin path is refused outside development, honouring the proxy's
      forwarded-proto as Traefik sets it — and the development exemption is asserted too, so it is a known hole
      rather than an accident
- [x] `no-store` and `noindex` on every admin response including the refusals — asserted for both the
      authorised and the refused case
- [x] Repeated wrong credentials from one IP are rate limited. 30/hour against a generated password is not a
      brute-force threat: the limiter's job is to make the *password's strength* the thing that matters rather
      than the attacker's bandwidth
- [x] A correct credential **is** throttled by an attacker sharing the IP — documented rather than fixed, in
      `allowAdminAttempt`, with the reasoning that the alternative (no limit on a shared password) is the
      weaker position, and a test that fails if the behaviour changes silently
- [x] The comparison remains constant-time after the limiter is added, and additionally does not
      short-circuit on the username — asserted structurally
- [x] Rate-limit rejections are logged with the client IP — confirmed in the live logs
