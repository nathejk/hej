# 005 — Auth endpoint: `POST /api/auth/verify` (session)

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Implement the PIN-verification step, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Given `{ phone, pin }`,
verify the stored hash, expiry, and attempt count; on success establish a
session and return identity + role.

Policy: lock out after **5** failed attempts; session lasts **≥ 7 days** via a
secure, HttpOnly, SameSite cookie carrying user id + role; the PIN is deleted on
success (single-use).

Depends on: 004.

## Acceptance Criteria

- [x] `POST /api/auth/verify` accepts `{ phone, pin }`, verifies
      hash/expiry/attempts.
- [x] Success: creates a secure HttpOnly SameSite session cookie (≥ 7-day
      lifetime, HMAC-signed), returns `{ user_id, role }`.
- [x] Failure: ErrNoPIN/ErrExpired/ErrMismatch → 401 (indistinguishable);
      lockout after 5 attempts → 429.
- [x] Endpoint has OpenAPI (swaggo) annotations.
- [x] Unit tests cover success, wrong PIN, no-PIN, plus session round-trip /
      tamper / expiry / wrong-secret.
- [x] `go test`, `go vet`, `staticcheck`, `gofmt` pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 18:00 — Implemented. New `internal/session`: stateless, HMAC-SHA256-signed cookie (`hej_session`) carrying `{uid, role, exp}`; `Manager.Issue/Read/Clear`; HttpOnly + SameSite=Lax + configurable Secure. Added `verifyPinHandler` (`cmd/api/auth.go`) with swag annotations, `InvalidCredentialsResponse` (401) helper, and config `SESSION_SECRET` + `SESSION_SECURE` (with `envBool`). Wired `sessions` onto the application struct (`7*24h` TTL) in `main.go` + test helper; registered `POST /api/auth/verify`.
- 2026-07-30 18:00 — Decision: **stateless signed cookie** rather than a server-side session store — survives the dev hot-reload restarts and needs no storage for the skeleton. Trade-off: logout is cookie-clear only (no server-side revocation / denylist); acceptable for now, noted for a future task if revocation is needed. `SESSION_SECRET` has an insecure dev default and MUST be overridden in prod (`docker-compose.override.yml`).
- 2026-07-30 18:00 — Decision: verify maps all of no-PIN / expired / mismatch to a single 401 (anti-enumeration); only attempt-lockout returns 429. Since a PIN only exists for a recognized number, a successful verify implies a valid user lookup.
- 2026-07-30 18:02 — ✅ Gates green (build/vet/test/gofmt/staticcheck). Tests: session package (round-trip/no-cookie/tampered/expired/wrong-secret) + verify handler (success sets cookie & returns role, wrong PIN → 401 no cookie, no-PIN → 401).
- 2026-07-30 18:02 — Completed. `session.Manager.Read`/`Clear` are ready for task 006 (`GET /api/me` + logout + auth middleware).
