# 006 — Auth endpoints: `GET /api/me` + `POST /api/auth/logout`

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Expose the session to the frontend and allow sign-out, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. `GET /api/me` returns the
current identity + role, or 401 when unauthenticated (the app uses this on load
to choose login vs shell). `POST /api/auth/logout` clears the session.

Depends on: 005.

## Acceptance Criteria

- [ ] `GET /api/me` returns `{ identity, role }` for a valid session, 401
      otherwise.
- [ ] `POST /api/auth/logout` clears the session cookie/record.
- [ ] Both endpoints have OpenAPI annotations.
- [ ] A reusable auth middleware/helper resolves the session for protected
      handlers (foundation for authorizing future data endpoints).
- [ ] `go test`, `go vet`, `staticcheck` pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
