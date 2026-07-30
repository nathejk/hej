# 020 — Version/update detection + `UpdatePrompt`

**Status:** open
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Let the app detect a newly released version and reload into it, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Use the `vite-plugin-pwa`
service-worker `needRefresh` flow: when a new SW is waiting, show a
non-blocking prompt; on confirm, `skipWaiting` + reload. No `/api/version`
endpoint needed.

Depends on: 014.

## Acceptance Criteria

- [ ] `registerSW`/`needRefresh` wired to app state (e.g. `app.store`).
- [ ] `src/components/UpdatePrompt.vue` shows a dismissible "A new version is
      available — Reload" notice (Toast or banner above the nav), styled via the
      Lara preset.
- [ ] Confirm activates the waiting SW (`skipWaiting`) and reloads into the new
      build.
- [ ] Dismiss keeps the app working; prompt can reappear.

## Progress Log

- 2026-07-30 13:12 — Task created.
