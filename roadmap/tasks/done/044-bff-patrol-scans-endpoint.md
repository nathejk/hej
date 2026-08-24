# 044 — BFF: GET /api/patrol/scans

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5, delegated sub-agent)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

The endpoint behind the map's registrations: the signed-in user's patrol's
checkpoint scans and bandit catches, newest first (PRD 002). Real data source is a
later dependency, so this ships against a seeded mock behind an interface — the
same pattern `internal/users` used in PRD 001.

## Acceptance Criteria

- [x] `internal/scans` package: `Kind` (`checkpoint`/`bandit`), `Scan`, and a
      `Source` interface with a seeded mock.
- [x] `GET /api/patrol/scans` behind `app.requireAuth`, wired in `routes.go`.
- [x] `lat`/`lng` nullable — a manually registered scan has no position.
- [x] Users with no patrol get `200` + an empty list, not `404`.
- [x] **OpenAPI annotations** matching the existing handlers' style.
- [x] Handler tests: 401 unauthenticated, 200 + scans, 200 + empty list.
- [x] `go build`, `go test`, `go vet`, `staticcheck`, `gofmt -l` all clean.

## Progress Log

- 2026-08-24 12:10 — Delegated alongside task 043.
- 2026-08-24 12:35 — Done. Response is
  `{"scans":[{"id","kind","label","lat","lng","scanned_at"}]}` with `lat`/`lng`
  nullable and `scans` always an array, never `null`. The mock sorts newest-first
  at construction and returns a copy so callers cannot mutate the fixture.
- 2026-08-24 12:36 — Seeded fixture for `mock-patrol-1042` ("Patrulje Ravnene"):
  four checkpoint scans across an evening plus one bandit catch, one of them
  deliberately **without coordinates** so the frontend's "listed but not plotted"
  path is exercised. Coordinates sit in central Jutland, near the map's default
  view.
- 2026-08-24 12:38 — **Dev-loop bug surfaced while verifying** (see task 048): the
  new route 404'd against a stale binary because `docker/init/api-dev` had not
  actually replaced the running process. `docker compose restart api` fixed it.
  Worth knowing before debugging a "missing" endpoint.
- 2026-08-24 12:40 — ✅ Verified live through Traefik: 401 without a cookie, the
  five seeded rows newest-first as the seeded spejder, and `{"scans":[]}` as the
  seeded postmandskab.
- 2026-08-24 13:05 — ✅ Confirmed end to end from the browser: the map plots 4
  markers (5 scans minus the un-positioned one) and the list shows all 5.
