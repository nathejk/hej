# 005 — Auth endpoint: `POST /api/auth/verify` (session)

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `POST /api/auth/verify` accepts `{ phone, pin }`, verifies
      hash/expiry/attempts.
- [ ] Success: creates a secure HttpOnly SameSite session cookie (≥ 7-day
      lifetime), deletes the PIN, returns `{ identity, role }`.
- [ ] Failure: increments attempts; locks out after 5; returns a clear,
      non-revealing error.
- [ ] Endpoint has OpenAPI annotations.
- [ ] Unit tests cover success, wrong PIN, expired PIN, lockout.
- [ ] `go test`, `go vet`, `staticcheck` pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
