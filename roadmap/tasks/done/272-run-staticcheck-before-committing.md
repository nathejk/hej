# 272 — Run `staticcheck` before committing Go changes

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

**Incident, and the process fix.** During PRD 016's phase 1–3 work the API stopped starting, presenting
to the user as a wall of Vite proxy errors:

```
[vite] http proxy error: /api/config
Error: connect ECONNREFUSED 172.19.0.4:4000
```

The cause was an unused test helper — `func setID` in `nathejk/table/kort/consumer_test.go`, left behind
when the test that needed it was rewritten. `staticcheck` reports that as `U1000`, and
`docker/init/api-dev` runs it as a **build gate**:

```
go get -v ./... && go test ./... && go vet ./... && go tool staticcheck ./... && go build ./... ...
```

A failing gate means `==> build failed; waiting for changes` and **the API is never started**. So a dead
test helper took the whole backend down for the running dev environment.

Every task in this PRD was verified with `go build ./...`, `go vet ./...`, `gofmt` and `go test ./...` —
none of which catches an unused identifier. The gate list was in the repo the whole time; I did not read
it before deciding what "clean" meant.

**The fix is a habit, not code:** run the same gates the container runs, in the same order, before
committing Go changes.

```sh
cd go && go test ./... && go vet ./... && go tool staticcheck ./... && go build ./...
```

`staticcheck` is pinned as a Go tool dependency, so `go tool staticcheck` needs no install.

Note `gosec` and `govulncheck` are pinned too but are **not** dev-loop gates (they run in the image
build); `gosec` currently reports 22 pre-existing findings, so it is not a signal that a change is bad.

## Acceptance Criteria

- [x] The unused helper removed and `go tool staticcheck ./...` clean.
- [x] API confirmed serving again (`/api/config` → 200, `/api/checkpoints` → 401 unauthenticated).
- [x] The gate list recorded here so the next session does not have to rediscover it.
- [x] `go-bff-layout` skill updated with the gate command, since that is where an agent looks before
      touching Go.

## Progress Log

- 2026-09-15 — Reported by the user as `ECONNREFUSED` from the Vite proxy. Diagnosed from
  `docker compose logs api`: `nathejk/table/kort/consumer_test.go:30:6: func setID is unused (U1000)`
  followed by `==> build failed; waiting for changes`.
- 2026-09-15 — Removed the helper; `staticcheck` clean; the dev loop rebuilt and started the API on its
  own. Verified with curl, and the new boot-time readiness line is visible in the log
  (`"no map sheets defined for this year yet","year":"2026","sets":0` — correct for this database, which
  has no `kort` events).
- 2026-09-15 — Root cause is process, not the helper: I validated with build/vet/test/gofmt and treated
  that as complete, without checking what the dev loop actually gates on. `staticcheck` was the one gate
  I was not running, and it is the one that stops the API.
- 2026-09-15 — Added the gate command to the `go-bff-layout` skill so it is found before the next Go
  change rather than after the next outage.
- 2026-09-15 — Done.
