# 021 — Embed build/version id

**Status:** done
**Priority:** low
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Embed a build/version identifier so both the app and the BFF know their own
version, per `roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Frontend via
Vite `define` (git SHA or `package.json` version); Go binary via `internal/vcs`.

Depends on: 001, 002.

## Acceptance Criteria

- [x] Frontend exposes a build id via Vite `define` (`__APP_VERSION__` from
      `npm_package_version`), typed in `env.d.ts`.
- [x] Go binary reports a build version via `internal/vcs` (ldflags override or
      embedded VCS build info).
- [x] Version visible for support: logged at startup (Go `configuration loaded`
      + JS `console.info`), returned in `GET /api/healthcheck` `system_info`,
      and shown on the login screen footer.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 22:25 — Go: added `internal/vcs.Version()` (prefers `-ldflags -X nathejk.dk/internal/vcs.version`, else `debug.ReadBuildInfo` vcs.revision + dirty flag). Wired into startup log + healthcheck `system_info.version`. Frontend: Vite `define __APP_VERSION__`, `env.d.ts` decl, `console.info` at startup, and a `v<version>` footer on the login screen.
- 2026-07-30 22:26 — ✅ Go build/vet/test/gofmt/staticcheck green; frontend build + type-check clean.
- 2026-07-30 22:26 — Completed. This was the last open PRD-001 task — the skeleton is feature-complete.
