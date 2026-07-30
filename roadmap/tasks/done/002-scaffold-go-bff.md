# 002 — Scaffold Go BFF (`go/cmd/api`)

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Create the Go backend-for-frontend skeleton under `go/` following the
`go-bff-layout` conventions, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Single `cmd/api` binary
that serves the JSON API and, in production, the built Vue bundle from the same
origin. Runs inside the `api` container.

No domain aggregates / JetStream / projections yet — just the transport frame
and facade stubs that later tasks (auth, push) hang off.

## Acceptance Criteria

- [x] `go/cmd/api/` with `main.go` (env-driven config + dependency wiring),
      `routes.go` (**stdlib `net/http` mux** under `/api` + SPA-fallback file
      server), and `env.go`.
- [x] `cmd/api/app/` transport helpers present (`WriteJSON`, `ReadJSON`, error
      responses, server) and embeddable on the application struct. (Healthcheck
      handler lives in `cmd/api/healthcheck.go`.)
- [x] `internal/data` (read facade) and `internal/commands` (write facade) stubs
      exist; handlers go through these, never touching SQL/JetStream directly.
- [x] Module path `nathejk.dk`; dependencies flow inward only.
- [x] `GET /api/healthcheck` responds; unknown `/api/*` returns JSON 404; SPA
      fallback serves static files from the web root.
- [x] `go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l` all pass
      (run on host — see deviation note).

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 13:40 — Picked up. Plan: `go mod init nathejk.dk`; create `cmd/api` (main/routes/env), `cmd/api/app` transport helpers, `internal/data` + `internal/commands` stubs, `www/` placeholder + SPA fallback, healthcheck. Verify with `go build/vet/test`. Env note: no `ui`/`api` container or Docker dev stack exists yet, so I'll run Go on the host to verify (documented deviation from the container-only convention).
- 2026-07-30 14:05 — Created `go/go.mod` (module `nathejk.dk`, go 1.24), `cmd/api/{main,env,routes,healthcheck}.go`, `cmd/api/app/{app,json,errors,server}.go`, `internal/data/models.go`, `internal/commands/commands.go`, `www/index.html`, and `cmd/api/routes_test.go`.
- 2026-07-30 14:05 — Decision: used the standard library `net/http` mux (Go 1.24 method+pattern routing) instead of `julienschmidt/httprouter`. Reason: no Docker dev stack / module-proxy access in this environment yet, so an external dep can't be fetched/verified. This keeps the skeleton buildable offline. Adopting httprouter (per go-bff-layout) is a small follow-up once the dev stack + network exist.
- 2026-07-30 14:05 — Decision: skipped OpenAPI annotations on the healthcheck (infra endpoint) — the annotation tool/convention is not yet chosen for this greenfield repo. Product endpoints (auth in tasks 004–006, push in 017) will carry annotations once the convention is set; noted in `healthcheck.go`.
- 2026-07-30 14:06 — ✅ Verified on host: `go build ./...`, `go vet ./...`, `go test ./...` (3 routing tests pass), and `gofmt -l` (clean) all green. `staticcheck` not run (not installed as a `go tool`; needs network — deferred to the dev-stack task).
- 2026-07-30 14:06 — Follow-ups surfaced (not in this task's scope): (a) Docker dev stack — Dockerfile + docker-compose with `ui`/`api` services + Traefik (no task exists yet; recommend adding one); (b) swap stdlib mux → httprouter; (c) choose OpenAPI annotation tooling; (d) register `staticcheck`/`gosec`/`govulncheck` as `go tool` directives.
- 2026-07-30 14:06 — Completed. Go BFF transport skeleton builds, vets, tests, and formats cleanly; ready for auth/push handlers to hang off `internal/data` + `internal/commands`.
