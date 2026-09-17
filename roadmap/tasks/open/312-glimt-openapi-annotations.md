# 312 — OpenAPI annotations for every Glimt endpoint

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

`.rules`: **all endpoints must have OpenAPI annotations.** swaggo-style comments directly above
each handler, in the style of `updatePhotoHandler` in `go/cmd/api/photo.go`: `@Summary`,
`@Description`, `@Tags`, `@Accept`, `@Produce`, `@Success`, every realistic `@Failure`, and
`@Router` (paths relative to `/api`).

Covers every endpoint from tasks 303–308, 319 and 323, including the unauthenticated public
ones — which should say explicitly that they ignore the session cookie, because that is a
deliberate property and not an oversight (PRD 019 §8).

Document the failure codes that actually exist: 400, 401, 403, 404, 413, 429, 503.

## Acceptance Criteria

- [ ] Every Glimt handler has a complete annotation block
- [ ] Public endpoints document that they are unauthenticated and ignore the session
- [ ] `@Tags` group them coherently (`glimt`, `glimt-moderation`, `glimt-public`)
- [ ] Failure codes match what the handlers actually return
- [ ] No endpoint added by PRD 019 is missing annotations

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
