# 002 — Scaffold Go BFF (`go/cmd/api`)

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Create the Go backend-for-frontend skeleton under `go/` following the
`go-bff-layout` conventions, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Single `cmd/api` binary
that serves the JSON API and, in production, the built Vue bundle from the same
origin. Runs inside the `api` container.

No domain aggregates / JetStream / projections yet — just the transport frame
and facade stubs that later tasks (auth, push) hang off.

## Acceptance Criteria

- [ ] `go/cmd/api/` with `main.go` (env-driven config + dependency wiring),
      `routes.go` (httprouter under `/api` + SPA-fallback file server), and
      `env.go`.
- [ ] `cmd/api/app/` transport helpers present (`WriteJSON`, `ReadJSON`, error
      responses, server, healthcheck) and embeddable on the application struct.
- [ ] `internal/data` (read facade) and `internal/commands` (write facade) stubs
      exist; handlers go through these, never touching SQL/JetStream directly.
- [ ] Module path `nathejk.dk`; dependencies flow inward only.
- [ ] `GET /api/healthcheck` (or equivalent) responds; SPA fallback serves
      static files in prod mode.
- [ ] `docker compose run --rm api go build ./...` and `go vet ./...` pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
