# 018 — Provision VAPID key pair + expose public key

**Status:** done
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Provision the Web Push VAPID key pair, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. The public key is shipped
to the client (baked into the build or served via `GET /api/push/public-key`);
the private key is held by the BFF as a secret (env var) for later delivery.

Depends on: 002.

## Acceptance Criteria

- [x] VAPID key pair provisioned via config: `VAPID_PUBLIC_KEY` /
      `VAPID_PRIVATE_KEY` env (empty defaults; documented in
      `docker-compose.yml`, private key belongs in the gitignored override).
- [x] Public key available to the client via `GET /api/push/public-key`
      (OpenAPI-annotated); returns empty when push isn't configured.
- [x] Rotation documented (regenerate a pair, update the override / prod env).
- [x] `go build`/`vet`/`test`/`gofmt`/`staticcheck` + `docker compose config`
      pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 21:05 — Added `vapidPublicKey`/`vapidPrivateKey` to config (env `VAPID_PUBLIC_KEY`/`VAPID_PRIVATE_KEY`), `GET /api/push/public-key` handler (`cmd/api/push.go`, swag-annotated) returning the configured public key, and route registration. Documented generation (`npx web-push generate-vapid-keys` or any VAPID generator) + secret handling in `docker-compose.yml` (private key → override file).
- 2026-07-30 21:05 — Decision: no real keypair committed (private key is a secret; empty dev defaults). Chose the endpoint over baking the public key into the Vite build so it can change without a frontend rebuild. Actual push *delivery* (signing with the private key) is a later PRD; this task only provisions + exposes.
- 2026-07-30 21:06 — ✅ Gates green + compose valid. Test asserts the endpoint echoes the configured public key.
- 2026-07-30 21:06 — Completed. Public key endpoint ready for the frontend push subscription (task 016); subscription storage is task 017.
