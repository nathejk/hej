# 015 — Geolocation permission service/store

**Status:** done
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Add a permission-aware geolocation service/store, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Wrap
`navigator.geolocation` (+ `navigator.permissions` for status) to request
access, read current position, expose it to the app, and handle
granted/denied/unavailable gracefully. No map rendering here (out of scope for
the skeleton). Location is not persisted server-side.

Depends on: 001.

## Acceptance Criteria

- [x] `src/stores/location.store.ts` exposes `permission` + `position` state,
      an `available` getter, and `syncPermission()` / `request()` actions.
- [x] Handles granted / denied / unavailable without breaking (request()
      resolves to null rather than rejecting on failure/denial).
- [x] Requested on demand (not on cold paint) — the contextual pre-prompt that
      calls `request()` is task 019.
- [x] Documented that it needs a secure context (HTTPS).

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 21:35 — Implemented `location.store` wrapping `navigator.geolocation` (+ `navigator.permissions` for non-prompting status with `onchange`). `request()` reads current position with sane options and maps `PERMISSION_DENIED` → `denied`; never rejects. Map rendering that consumes `position` is a later feature PRD.
- 2026-07-30 21:36 — ✅ Verified in `node:20-alpine`: build + type-check clean.
- 2026-07-30 21:36 — Completed. Ready for the contextual location pre-prompt (task 019, on Maps).
