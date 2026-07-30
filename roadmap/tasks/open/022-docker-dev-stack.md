# 022 — Docker dev stack (Dockerfile + docker-compose + Traefik)

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Create the containerised dev/prod stack for the repo, following the
`docker-dev-stack` skill and the key files listed in `.rules`. This is a
prerequisite for building/running the frontend (task 001 needs `npm` in the
`ui` container) and for running the BFF the conventional way (`api` container),
and it unblocks the container-based validation that tasks 002+ had to run on the
host instead.

Per `.rules` this repo has two services:

| Service  | Technology | Dev target | Local URL |
|----------|-----------|-----------|-----------|
| frontend | Vue 3 (TS) | `ui-dev`  | app dev host (confirm — e.g. `hej.local.nathejk.dk`) |
| bff      | Go         | `base`    | internal only |

The multistage `docker/Dockerfile` should provide dev targets `ui-dev` and
`base`, and a prod target `prod` (Go binary serving the built SPA from one
origin). Dev init scripts live under `docker/init/` (`ui-dev`, `api-dev`) with
hot reload (Vite for `ui`; `go run` + `inotifywait` restart for `api`, running
`go test` / `go vet` / `go tool staticcheck` / `go build` on `.go`/`.sql`
change).

References:
- Existing BFF: `go/cmd/api` (built with `go 1.24`; dev image should match).
- Existing SPA placeholder served by the BFF: `go/www/`.
- PRD: `roadmap/prd/001-hej-nathejk-event-app-skeleton.md` (HTTPS/secure-context
  requirement for service workers, geolocation, notifications).

## Acceptance Criteria

- [ ] `docker/Dockerfile` multistage build with dev targets `ui-dev` and `base`
      and a `prod` target that produces a static Go binary serving the built
      SPA.
- [ ] `docker-compose.yml` defining the `ui` and `api` services, a named volume
      for `ui` `node_modules`, and env vars (with dev defaults) consumed by the
      BFF (`PORT`, `ENV`, `WEB_ROOT`).
- [ ] Traefik routing labels serving the frontend over **HTTPS** (secure context
      required for SW/geolocation/notifications), with `/api` reaching the BFF.
- [ ] `docker/init/ui-dev` (npm ci + `npm run dev`) and `docker/init/api-dev`
      (test/vet/staticcheck/build + `go run nathejk.dk/cmd/api`, restart on
      change) present.
- [ ] Confirm the app's dev hostname (update `.rules` if it should differ from
      the tilmelding default).
- [ ] `docker compose build` succeeds; `docker compose up` serves the SPA over
      HTTPS and proxies `/api/healthcheck` to the BFF.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-07-30 14:10 — Task created. Surfaced during task 002 (Go BFF scaffold): no Docker dev stack exists yet, which blocks container-based frontend build/run and conventional BFF runs. Also lets tasks 004+ swap stdlib mux for httprouter and register `go tool` linters once module-proxy/network is available in-container.
