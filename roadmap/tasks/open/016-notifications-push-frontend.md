# 016 — Notification permission + Web Push subscription (frontend)

**Status:** open
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] Notification permission requested contextually (behind task 019
      pre-prompt); denial does not break the app.
- [ ] On grant, a push subscription is created via the SW and POSTed to
      `POST /api/push/subscription`.
- [ ] Service worker handles `push` (show notification) and `notificationclick`.
- [ ] Subscription tied to the logged-in user (session available).

## Progress Log

- 2026-07-30 13:12 — Task created.
