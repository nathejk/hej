# 006 — Auth endpoints: `GET /api/me` + `POST /api/auth/logout`

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Expose the session to the frontend and allow sign-out, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. `GET /api/me` returns the
current identity + role, or 401 when unauthenticated (the app uses this on load
to choose login vs shell). `POST /api/auth/logout` clears the session.

Depends on: 005.

## Acceptance Criteria

- [x] `GET /api/me` returns `{ user_id, role }` for a valid session, 401
      otherwise.
- [x] `POST /api/auth/logout` clears the session cookie (idempotent).
- [x] Both endpoints have OpenAPI (swaggo) annotations.
- [x] A reusable `requireAuth` middleware resolves the session and puts it on the
      request context (foundation for authorizing future protected endpoints).
- [x] `go test`, `go vet`, `staticcheck`, `gofmt` pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 18:20 — Implemented. Added `cmd/api/context.go` (session context key + get/set), `cmd/api/middleware.go` (`requireAuth` — 401 when no valid session, else session on context), `meHandler` + `logoutHandler` (`cmd/api/auth.go`, swag-annotated), and `AuthenticationRequiredResponse` (401). Registered `GET /api/me` (behind `requireAuth`) and `POST /api/auth/logout`.
- 2026-07-30 18:21 — ✅ Gates green (build/vet/test/gofmt/staticcheck). Tests: `/api/me` 401 without session; full verify→cookie→`/api/me` returns role `spejder`; logout returns 200 and clears the cookie (MaxAge < 0).
- 2026-07-30 18:21 — Completed. Auth backend is end-to-end: request-pin → verify (session) → me / logout, with `requireAuth` ready to gate future protected/role-scoped endpoints. Frontend can now build the session store + guards (task 008) and login view (task 007).
