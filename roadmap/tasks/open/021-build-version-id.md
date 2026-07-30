# 021 — Embed build/version id

**Status:** open
**Priority:** low
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Embed a build/version identifier so both the app and the BFF know their own
version, per `roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Frontend via
Vite `define` (git SHA or `package.json` version); Go binary via `internal/vcs`.

Depends on: 001, 002.

## Acceptance Criteria

- [ ] Frontend exposes a build id (Vite `define`) accessible at runtime.
- [ ] Go binary reports a build version via `internal/vcs`.
- [ ] Version is visible somewhere for support (e.g. logged at startup and/or
      shown in a small "about"/settings spot).

## Progress Log

- 2026-07-30 13:12 — Task created.
