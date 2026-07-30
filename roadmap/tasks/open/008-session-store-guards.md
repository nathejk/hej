# 008 — Session store, fetchWrapper & router guards

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Wire client-side auth state and gating, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. On load the app calls
`GET /api/me`; 401 → show login, otherwise populate `session.store` and render
the shell. A global router guard enforces auth and role; `fetchWrapper` treats
any 401 as "session lost" and routes back to login.

Note: client-side role gating is UX only — the BFF authorizes protected
endpoints independently.

Depends on: 001, 006.

## Acceptance Criteria

- [ ] `src/stores/session.store.ts` holds identity + role and is hydrated from
      `GET /api/me` on startup.
- [ ] `@/helpers` `fetchWrapper` attaches the session and redirects to login on
      any 401.
- [ ] Global router guard: unauthenticated → login; role not permitted for a
      route → redirect to an allowed page.
- [ ] Anonymous/unauthenticated is a valid handled state (only login reachable).

## Progress Log

- 2026-07-30 13:12 — Task created.
