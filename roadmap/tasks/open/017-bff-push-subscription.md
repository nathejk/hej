# 017 — BFF `POST /api/push/subscription` + storage

**Status:** open
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Store client Web Push subscriptions on the BFF, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`, so event updates can be
delivered later (delivery itself is a later PRD). Subscriptions are tied to the
logged-in user (session from task 006).

Depends on: 002, 006.

## Acceptance Criteria

- [ ] `POST /api/push/subscription` accepts a subscription (endpoint, p256dh,
      auth) and persists it with `created_at` + user id from the session.
- [ ] Requires an authenticated session.
- [ ] Idempotent on repeat subscription for the same user/endpoint.
- [ ] Endpoint has OpenAPI annotations.
- [ ] `go test`, `go vet`, `staticcheck` pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
