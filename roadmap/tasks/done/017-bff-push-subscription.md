# 017 — BFF `POST /api/push/subscription` + storage

**Status:** done
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Store client Web Push subscriptions on the BFF, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`, so event updates can be
delivered later (delivery itself is a later PRD). Subscriptions are tied to the
logged-in user (session from task 006).

Depends on: 002, 006.

## Acceptance Criteria

- [x] `POST /api/push/subscription` accepts a subscription (endpoint + keys) and
      persists it with `created_at` + the session user id.
- [x] Requires an authenticated session (`requireAuth`).
- [x] Idempotent per (user, endpoint) — re-subscribing replaces, doesn't
      duplicate.
- [x] Endpoint has OpenAPI (swaggo) annotations.
- [x] `go test`/`vet`/`staticcheck`/`gofmt` pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 21:20 — Implemented `internal/push` (`Subscription`, `Store` interface, `MemoryStore` keyed by `userID|endpoint`) and `createPushSubscriptionHandler` (`cmd/api/push.go`, swag-annotated, behind `requireAuth`, validates endpoint+keys). Wired `pushStore` onto the application struct + `main.go` + test helper; registered `POST /api/push/subscription`.
- 2026-07-30 21:20 — Decision: in-memory store keyed by (user, endpoint) for the skeleton (survives per-process; a MariaDB-backed `Store` can replace it later). Subscriptions are tied to the session user (per PRD push identity). Delivery/fan-out remains a later PRD.
- 2026-07-30 21:21 — ✅ Gates green. Tests: 401 without session; authed → 201 and idempotent (2 identical posts → 1 stored); missing keys → 400.
- 2026-07-30 21:21 — Completed. BFF side of push ready; frontend subscribe flow is task 016.
