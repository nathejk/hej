# 016 — Notification permission + Web Push subscription (frontend)

**Status:** done
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Request notification permission and register a Web Push subscription, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Subscribe via the service
worker (`PushManager.subscribe({ userVisibleOnly: true, applicationServerKey }`)
using the VAPID public key (task 018) and POST the subscription to the BFF (task
017). The SW must handle `push` and `notificationclick`. Scope is
**capture-only**: delivery/fan-out is a later PRD.

iOS caveat: Web Push only works when installed to the home screen.

Depends on: 014, 017, 018.

## Acceptance Criteria

- [x] Notification permission requested via `notifications.store.enable()`
      (contextual pre-prompt is task 019); denial degrades gracefully (returns
      false, never throws).
- [x] On grant: subscribes via the SW `pushManager` using the VAPID key from
      `GET /api/push/public-key`, then POSTs `{endpoint, keys}` to
      `POST /api/push/subscription`.
- [x] Service worker handles `push` (showNotification) and `notificationclick`
      (focus/open) via `public/push-sw.js`, imported by the Workbox SW
      (`workbox.importScripts`).
- [x] Subscription tied to the logged-in user (BFF associates it with the
      session).

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 21:55 — Implemented `notifications.store` (`available` getter, `syncPermission`, `enable()` — requestPermission → fetch VAPID public key → `pushManager.subscribe` → POST to BFF; base64→Uint8Array helper). Added `public/push-sw.js` (push + notificationclick handlers) and wired `workbox.importScripts: ['push-sw.js']` in `vite.config.ts` so the generateSW Workbox SW pulls them in.
- 2026-07-30 21:56 — Fix: newer TS types `Uint8Array<ArrayBufferLike>` which isn't assignable to `applicationServerKey` (wants ArrayBuffer-backed); constructed the array on an explicit `new ArrayBuffer(...)` and annotated `Uint8Array<ArrayBuffer>`.
- 2026-07-30 21:57 — ✅ Verified in `node:20-alpine`: build (sw.js emitted, push-sw imported) + type-check clean.
- 2026-07-30 21:57 — Completed. Notification/push subscribe flow ready; the contextual pre-prompt on Updates is task 019. (Live push needs configured VAPID keys + HTTPS + a device.)
