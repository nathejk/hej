# 020 — Version/update detection + `UpdatePrompt`

**Status:** done
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Let the app detect a newly released version and reload into it, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Use the `vite-plugin-pwa`
service-worker `needRefresh` flow: when a new SW is waiting, show a
non-blocking prompt; on confirm, `skipWaiting` + reload. No `/api/version`
endpoint needed.

Depends on: 014.

## Acceptance Criteria

- [x] `registerSW`/`onNeedRefresh` wired to `app.store.updateAvailable`
      (via `@/helpers/pwa` `initPwa`, done in task 014).
- [x] `src/components/UpdatePrompt.vue` shows a dismissible "En ny version er
      tilgængelig — Genindlæs" banner (Teleported, top, safe-area aware),
      re-appears if a newer version is announced after dismissal.
- [x] Confirm activates the waiting SW + reloads via `applyUpdate()`
      (`registerSW` reload).
- [x] Dismiss ("Senere") keeps the app working; prompt can reappear.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 20:55 — Implemented `UpdatePrompt.vue` (reads `app.store.updateAvailable` via `storeToRefs`, `applyUpdate()` on confirm, local dismiss that resets when a new version is announced). Mounted it at the App root so it shows over both the shell and login. Detection side was already wired in task 014 (`initPwa` → `setUpdateAvailable`).
- 2026-07-30 20:56 — ✅ Verified in `node:20-alpine`: build (sw.js emitted) + type-check clean.
- 2026-07-30 20:56 — Completed. Full update loop present: SW detects waiting build → banner → reload into new version. (End-to-end SW update behaviour needs the HTTPS deploy to exercise live.)
