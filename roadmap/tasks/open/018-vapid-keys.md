# 018 — Provision VAPID key pair + expose public key

**Status:** open
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Provision the Web Push VAPID key pair, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. The public key is shipped
to the client (baked into the build or served via `GET /api/push/public-key`);
the private key is held by the BFF as a secret (env var) for later delivery.

Depends on: 002.

## Acceptance Criteria

- [ ] VAPID key pair generated; private key read from an env var (documented in
      `docker-compose.yml` with a dev default) and never committed.
- [ ] Public key available to the client (baked-in via Vite `define` or
      `GET /api/push/public-key` with OpenAPI annotations).
- [ ] Documented how to rotate/replace keys.

## Progress Log

- 2026-07-30 13:12 — Task created.
