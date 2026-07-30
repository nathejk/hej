# 019 — Soft permission pre-prompts (`PermissionPrompt`)

**Status:** open
**Priority:** low
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Add soft in-app pre-prompts that explain *why* the app wants location /
notifications before triggering the native browser prompt, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Shown contextually after
login (location on Maps, notifications on Updates) so a decline doesn't
permanently burn the permission.

Depends on: 010, 015, 016.

## Acceptance Criteria

- [ ] `src/components/PermissionPrompt.vue` with clear rationale copy per
      permission.
- [ ] Location pre-prompt surfaced on Maps; notifications pre-prompt on Updates.
- [ ] Native prompt only fires after the user accepts the pre-prompt.
- [ ] Declines are remembered (e.g. `localStorage`) to avoid nagging; a way to
      re-request remains available.

## Progress Log

- 2026-07-30 13:12 — Task created.
