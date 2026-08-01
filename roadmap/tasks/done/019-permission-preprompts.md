# 019 — Soft permission pre-prompts (`PermissionPrompt`)

**Status:** done
**Priority:** low
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Add soft in-app pre-prompts that explain *why* the app wants location /
notifications before triggering the native browser prompt, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Shown contextually after
login (location on Maps, notifications on Updates) so a decline doesn't
permanently burn the permission.

Depends on: 010, 015, 016.

## Acceptance Criteria

- [x] `src/components/PermissionPrompt.vue` with rationale copy (title/message/
      cta/icon props; accept/dismiss emits).
- [x] Location pre-prompt surfaced on Maps; notifications pre-prompt on Updates.
- [x] Native prompt only fires after the user accepts the pre-prompt
      (`accept` → `location.request()` / `notifications.enable()`).
- [x] Declines remembered in `localStorage` (per-permission key) so they don't
      nag; re-requesting still possible by clearing the flag / via OS settings.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 22:10 — Implemented reusable `PermissionPrompt.vue` (Lucide icon, rationale, accept/"Ikke nu"). Wired it into `MapsView` (location, `MapPin`) and `UpdatesView` (notifications, `Bell`): each shows only when the permission is still gainable (`unknown`/`prompt`/`default`), not already granted/subscribed, and not previously dismissed (localStorage key per permission). Accept triggers the store action (which fires the native prompt).
- 2026-07-30 22:11 — ✅ Verified in `node:20-alpine`: build + type-check clean.
- 2026-07-30 22:11 — Completed. Contextual, non-nagging permission UX in place for both location and notifications.
