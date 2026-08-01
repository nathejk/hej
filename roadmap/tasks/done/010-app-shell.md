# 010 — App shell (`App.vue`) + top app bar

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Build the mobile app shell, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`: a full-height, safe-area
aware layout with a scrollable content region (`<router-view/>`) and a fixed
bottom navigation bar. Only rendered when authenticated; the login screen shows
otherwise.

Include a minimal top app bar that hosts (at least) **sign-out** (calls
`POST /api/auth/logout` and returns to login).

Depends on: 001, 006, 008.

## Acceptance Criteria

- [x] `App.vue` renders the shell (top bar + scrollable content + fixed bottom
      nav) only when authenticated; the login screen renders bare.
- [x] Safe-area insets respected (top bar `env(safe-area-inset-top)`, nav
      `env(safe-area-inset-bottom)`); content is the only scroll region
      (`flex-1 overflow-y-auto`), nav is fixed at the bottom of the column.
- [x] Minimal top app bar with brand + working "Log ud" (calls
      `session.logout()` → `POST /api/auth/logout`, then routes to login).
- [x] Nav tap targets ≥ 44px (`min-h-[3.25rem]`).

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 19:40 — Implemented `App.vue` shell (flex column: top app bar with brand + sign-out, `overflow-y-auto` content with `<RouterView/>`, fixed `<BottomNav/>`), gated on `session.isAuthenticated`. Added a first-cut `BottomNav.vue` rendering role-visible destinations (`visibleDestinations`) with active-route highlighting. The ≤5/burger overflow rule + MoreMenu sheet come in tasks 011/012.
- 2026-07-30 19:41 — ✅ Verified in `node:20-alpine`: `npm run build` + `npm run type-check` clean.
- 2026-07-30 19:41 — Completed. Shell frames the app; BottomNav ready to gain the burger rule (011).
