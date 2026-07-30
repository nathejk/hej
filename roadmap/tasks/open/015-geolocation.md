# 015 — Geolocation permission service/store

**Status:** open
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Add a permission-aware geolocation service/store, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Wrap
`navigator.geolocation` (+ `navigator.permissions` for status) to request
access, read current position, expose it to the app, and handle
granted/denied/unavailable gracefully. No map rendering here (out of scope for
the skeleton). Location is not persisted server-side.

Depends on: 001.

## Acceptance Criteria

- [ ] `src/stores/location.store.ts` (or a service) exposes permission status +
      current position and a `request()` action.
- [ ] Handles granted / denied / unavailable without breaking the app.
- [ ] Requested contextually (not on cold first paint) — behind the soft
      pre-prompt from task 019.
- [ ] Works only in a secure context (documented).

## Progress Log

- 2026-07-30 13:12 — Task created.
