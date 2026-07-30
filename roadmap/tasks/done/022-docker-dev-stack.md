# 022 — Docker dev stack (Dockerfile + docker-compose + Traefik)

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

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

- [x] `docker/Dockerfile` multistage build with dev targets `ui-dev` and `base`
      and a `prod` target that produces a static (CGO-free) Go binary serving the
      built SPA. (`api-dev→base→build`, `ui-dev→ui-builder`, `prod`.)
- [x] `docker-compose.yml` defining `ui` and `api` (plus `db` + `phpmyadmin` for
      upcoming auth/push storage), a named `ui-node_modules` volume, and BFF env
      with dev defaults (`PORT`, `ENV`, `WEB_ROOT`, `GOCACHE`).
- [x] Traefik labels serve the frontend over **HTTPS** (redirect pattern,
      `desec` cert) so the SPA runs in a secure context; `/api` reaches the BFF
      via the Vite dev proxy (api stays internal-only, no Traefik labels).
- [x] `docker/init/ui-dev` (npm ci + `npm run dev`) and `docker/init/api-dev`
      (gosec/govulncheck once report-only, then get/test/vet/staticcheck/build +
      `go run nathejk.dk/cmd/api`, `inotifywait` restart on `*.go`/`*.sql`)
      present.
- [x] Dev hostname confirmed: **`hej.local.nathejk.dk`** (repo-scoped router
      prefix `hej`, `sql.hej.local.nathejk.dk` for phpmyadmin). `.rules` updated
      (title, purpose, hostname) off the tilmelding default.
- [x] `docker compose config` valid; `docker build --target ui-dev` and
      `--target base` succeed; running the `base` image serves
      `/api/healthcheck` (200), SPA fallback (200), and unknown `/api/*` (JSON
      404). Full `docker compose up` needs the org infra repo's external
      `traefik` network + local DNS (documented) and the `ui` service needs
      `vue/` from task 001 — so those parts aren't runnable in this environment.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-07-30 14:10 — Task created. Surfaced during task 002 (Go BFF scaffold): no Docker dev stack exists yet, which blocks container-based frontend build/run and conventional BFF runs. Also lets tasks 004+ swap stdlib mux for httprouter and register `go tool` linters once module-proxy/network is available in-container.
- 2026-07-30 15:00 — Picked up. Plan: multistage `docker/Dockerfile` (api-dev→base→build, ui-dev→ui-builder, prod), `docker-compose.yml` (ui, api, db, phpmyadmin), `docker/init/{api-dev,ui-dev}`, root `.gitignore` for `docker-compose.override.yml`. Decisions to make: app dev host = `hej.local.nathejk.dk` (repo-scoped router prefix `hej`, avoids colliding with the real tilmelding repo on shared Traefik) — will update `.rules`. No shared-go/go.work (this repo doesn't use shared-go). No `mail`/`jetstream` (app uses SMS not SMTP, no NATS yet). Verify what's possible here: `docker compose config` + build the dev stages; note full `up` needs the infra repo's external `traefik` network + local DNS.
- 2026-07-30 15:20 — Wrote `docker/Dockerfile` (6 stages), `docker/init/api-dev` + `docker/init/ui-dev`, `docker-compose.yml`, root `.gitignore`, and updated `.rules`.
- 2026-07-30 15:20 — Decisions: (a) `CGO_ENABLED=0` static build (BFF is pure Go, no cgo) instead of the skill's `CGO_ENABLED=1 -extldflags static` — simpler and correct for alpine. (b) Omitted shared-go / `go.work` (unused). (c) Included `db` (mariadb:10.8) + `phpmyadmin` now since auth (PIN/session) + push subscriptions land next and will need storage; the BFF doesn't read DB env yet. (d) Dropped `mail`/mailhog — the app authenticates via SMS, not SMTP. (e) Binary path `/hej-api`, prod `SENTRY_RELEASE` from `BUILD_VERSION` arg for CI parity.
- 2026-07-30 15:22 — Updated `.rules`: title → "Hej Nathejk", purpose → in-event companion app, frontend Local URL → `https://hej.local.nathejk.dk`. (Flagging for user: rebranded the rules off the tilmelding template since this repo is the Hej Nathejk app.)
- 2026-07-30 15:30 — ✅ Verified: `sh -n` on both init scripts OK; `docker compose config` valid; `docker build --target ui-dev` OK; `docker build --target base` OK (go mod download ~30s). Ran the `base` image (`go run nathejk.dk/cmd/api`): `/api/healthcheck` → 200 JSON, `/` → 200 (SPA fallback from /app/www), `/api/unknown` → 404 JSON. httprouter + SPA fallback confirmed inside the container image.
- 2026-07-30 15:30 — Not runnable here (documented, not blocking): full `docker compose up` (needs infra repo's external `traefik` network + `*.local.nathejk.dk` DNS + the `redirect-to-https`/`desec` Traefik config), and the `ui` service (needs `vue/` from task 001). `prod`/`ui-builder` stages also depend on `vue/`.
- 2026-07-30 15:31 — Completed. Dev stack scaffolded and verified as far as the environment allows; unblocks task 001 (frontend scaffold) and gives auth/push tasks a `db` to target.
