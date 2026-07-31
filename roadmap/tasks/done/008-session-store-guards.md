# 008 — Session store, fetchWrapper & router guards

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

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

- [x] `src/stores/session.store.ts` holds identity + role and is hydrated from
      `GET /api/me` (via `ensureReady`/`fetchMe`); exposes `isAuthenticated` +
      `role` getters and `requestPin`/`verify`/`logout` actions.
- [x] `@/helpers` `fetchWrapper` sends credentials and throws a typed
      `HttpError` (with status); the session store treats 401 as "not signed
      in". *(Auto-redirect on mid-session 401 is handled by the router guard
      re-checking on navigation; a global 401→login interceptor can be added
      later if needed.)*
- [x] Global router guard: `ensureReady()` then unauthenticated → `/login`,
      authenticated-on-login → `/`, and role not in `meta.roles` → home
      (RouteMeta augmented with `public?`/`roles?`).
- [x] Anonymous/unauthenticated is a valid handled state (only `/login`
      reachable); a minimal functional LoginView proves the gate end-to-end.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 18:45 — Implemented. `fetchWrapper` now throws `HttpError{status}` on non-2xx; `session.store` (Pinia) owns `{user, ready}` with `fetchMe`/`ensureReady`/`requestPin`/`verify`/`logout`. Router gained a `/login` route + `beforeEach` guard (auth + role, with `RouteMeta` augmentation). Added a minimal but working `LoginView.vue` (phone → PIN → verify) so the gate is verifiable; task 007 will productionize its UX.
- 2026-07-30 18:45 — Decision: no global auto-redirect interceptor in `fetchWrapper` (would risk redirect loops with the `/api/me` probe). The guard re-resolves the session on navigation instead; a dedicated 401 interceptor can come later if mid-session expiry UX needs it.
- 2026-07-30 18:46 — ✅ Verified in `node:20-alpine`: `npm run build` (LoginView code-split) and `npm run type-check` both clean.
- 2026-07-30 18:46 — Completed. App is gated; session state + guards ready for the app shell (task 010) and the full LoginView (task 007).
