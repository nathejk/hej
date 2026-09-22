# 371 — HTTPS-only, `no-store`, `noindex`, and a rate limit on the guess

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

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

## Acceptance Criteria

- [ ] A plain-HTTP request to any admin path is refused outside development, and the check honours the
      proxy's forwarded-proto as Traefik sets it
- [ ] `X-Robots-Tag: noindex, nofollow` and `Cache-Control: no-store` are present on every admin
      response — HTML, JSON, media bytes and error responses alike, asserted by walking the admin route
      table rather than per handler
- [ ] Repeated wrong credentials from one IP are rate limited, and the limit is reached before a
      password of the configured length is brute-forceable at any useful speed
- [ ] A correct credential is not rate limited out of the way by an attacker hammering the same IP
      range — or, if it is, that tradeoff is documented
- [ ] The comparison remains constant-time after the limiter is added
- [ ] Rate-limit rejections are logged with the client IP
