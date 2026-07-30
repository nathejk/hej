# 010 — App shell (`App.vue`) + top app bar

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `App.vue` renders shell (content + fixed bottom nav) only when
      authenticated; login otherwise.
- [ ] Safe-area insets respected (top + bottom); no horizontal scroll; content
      scrolls independently of the fixed nav.
- [ ] Minimal top app bar with a working sign-out action.
- [ ] Tap targets ≥ 44px.

## Progress Log

- 2026-07-30 13:12 — Task created.
