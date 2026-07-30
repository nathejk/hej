# 023 — BFF conventions hardening: httprouter, OpenAPI tooling, go tool linters

**Status:** done
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Follow-ups surfaced by task 002 (Go BFF scaffold), now doable because
module-proxy/network access is available:

1. **Swap the stdlib `net/http` mux for `julienschmidt/httprouter`** to match the
   `go-bff-layout` convention, keeping the `/api/*` JSON-404 + SPA-fallback
   behaviour.
2. **Choose and set up the OpenAPI annotation tool.** Repo rule: all endpoints
   must have OpenAPI annotations. Pick a tool, register it, add the annotation
   convention (general API info + annotate the healthcheck as the worked
   example), and confirm generation works.
3. **Register `staticcheck`, `gosec`, and `govulncheck` as `go tool` directives**
   (Go 1.24+), per `go-bff-layout`, so they're version-pinned in `go.mod`.

Runs on host (no Docker dev stack yet — see task 022).

## Acceptance Criteria

- [x] `cmd/api/routes.go` uses `httprouter`; `/api/healthcheck` works, unknown
      `/api/*` returns JSON 404, non-API paths serve the SPA.
- [x] OpenAPI annotation tool chosen (**swaggo/swag**) and registered as a
      `go tool`; general API info (`main.go`) + healthcheck annotations added;
      generation command documented (`go tool swag init -g cmd/api/main.go -o
      cmd/api/docs`) and verified (spec captured `/healthcheck` + title). Spec is
      gitignored (kept tool-only; regenerated on demand).
- [x] `staticcheck`, `gosec`, `govulncheck` registered as `go tool` directives in
      `go.mod`.
- [x] `go build`, `go vet`, `go test`, `gofmt -l`, and `go tool staticcheck` all
      pass; `gosec` (0 issues) and `govulncheck` run (report-only).

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-07-30 14:20 — Task created + picked up. Network/module-proxy confirmed available (added `httprouter` to go.mod as first step). Plan: (1) rewrite routes.go on httprouter; (2) set up swaggo/swag as the OpenAPI tool + annotate; (3) register the three linters as go tools; verify all gates.
- 2026-07-30 14:35 — ✅ Rewrote `cmd/api/routes.go` on `httprouter`: `GET /api/healthcheck` registered; `router.NotFound` splits `/api/*` → JSON 404 vs everything else → SPA fallback; `router.MethodNotAllowed` → JSON 405. Existing routing tests still pass.
- 2026-07-30 14:45 — Decision: chose **swaggo/swag** as the OpenAPI annotation tool (de-facto Go standard, annotation-comment based, matches the repo's "OpenAPI annotations" rule). Registered `github.com/swaggo/swag/cmd/swag` as a `go tool`; added general `@title/@version/@description/@BasePath` info to `main.go` and `@Summary/@Success/@Router` etc. to the healthcheck. Verified `go tool swag init` generates a valid spec capturing `/healthcheck`.
- 2026-07-30 14:45 — Decision: keep the generated spec **out of git** (added `go/.gitignore` for `cmd/api/docs/`). Committing `docs.go` would make `swaggo/swag` a build dependency; keeping it tool-only is cleaner until we actually serve a Swagger UI. Regen command documented in `healthcheck.go` + `.gitignore`.
- 2026-07-30 14:46 — ✅ Registered `staticcheck`, `gosec`, `govulncheck` as `go tool` directives (`go get -tool ...@latest`). Consequence: `staticcheck`/`gosec` require Go ≥ 1.25, so `go.mod`'s `go` directive auto-bumped 1.24 → 1.25.8. This is consistent with the `golang:1.25` dev image from `docker-dev-stack`; flagging it since "bump Go as its own task" is a convention — here it was forced by the tool versions.
- 2026-07-30 14:47 — Note: `go tool` deps for gosec pull a large indirect tree (otel/grpc + some AI SDKs that gosec 2.28 depends on). These are tool-only build deps and don't affect the API binary's imports; kept as-is per the go-bff-layout convention to register these tools in `go.mod`.
- 2026-07-30 14:48 — ✅ Gates: `go build ./...`, `go vet ./...`, `go test ./...` (3 tests), `gofmt -l` (clean), `go tool staticcheck ./...` all green. `gosec` = 0 issues. `govulncheck` = report-only; 7 stdlib advisories all "Fixed in go1.25.9" (a Go patch bump, not a code issue). `httprouter` recorded as a direct require; four tools in the `tool` block.
- 2026-07-30 14:48 — Completed. BFF now matches go-bff-layout conventions for routing + linters + OpenAPI tooling. Follow-up (unblocked later): once the Docker dev stack (task 022) exists, wire the dev-loop to run these `go tool` gates, and optionally serve a Swagger UI.
