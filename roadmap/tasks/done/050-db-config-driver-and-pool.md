# 050 — DB config, MariaDB driver and pooled connection

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008. `hej`'s Go binary has no database connection at all today: `go.mod` has
two direct dependencies, and `cmd/api/main.go` / `config.go` contain no `sql`
reference. Add a `DB_DSN` config value, the MariaDB driver, and a pooled
`*sql.DB` opened at startup.

Follow `go-bff-layout`: read the env var in `main.go` via
`flag.StringVar(..., os.Getenv(...))` and pass it down through the `config`
struct — never read env vars deeper in the call tree.

The connection must not make the API unbootable when the database is briefly
unavailable: ping with a bounded retry and fail loudly if it never comes up, but
do not hang forever.

## Acceptance Criteria

- [x] `DB_DSN` read in `main.go` into the existing `config` struct
- [x] `github.com/go-sql-driver/mysql` added to `go.mod`
- [x] Pooled `*sql.DB` with explicit `SetMaxOpenConns` / `SetMaxIdleConns` /
      `SetConnMaxLifetime`
- [x] Startup ping with bounded retry and a clear log line on failure
- [x] Connection closed on shutdown
- [x] `go build ./...`, `go vet ./...`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Plan: config fields + `openDB` with bounded retry, held
  on `application` for shutdown and the future healthcheck.
- 2026-08-25 — Note: the `config` struct lives in `cmd/api/env.go`, not
  `cmd/api/config.go` (that file is the `GET /api/config` handler). Added the DB
  fields there alongside the existing env helpers, plus a new `envDuration`.
- 2026-08-25 — Decision: **an empty DSN and an unreachable database are both
  non-fatal.** Everything the API serves today comes from mocks, so refusing to
  boot would be a regression for no benefit. Logged loudly instead; task 059's
  healthcheck is what will make the state visible rather than silent. Once PRD
  006's directory lands on the login path this becomes a real outage — that is
  accepted deliberately in PRD 008 §5.
- 2026-08-25 — Decision: the ping **retries but is bounded** (`DB_CONNECT_TIMEOUT`,
  default 10s). The retry is for container start order, not flakiness: `depends_on`
  waits for the container, not for MariaDB inside it, so a single ping at t=0 would
  fail on every cold `docker compose up`. Unbounded waiting was rejected because an
  API blocked forever is indistinguishable from a hung deploy.
- 2026-08-25 — Decision: pool bounds are explicit (25/25/5m) rather than Go's
  defaults. Go defaults `MaxOpenConns` to unlimited while MariaDB defaults
  `max_connections` to 151, so an unbounded pool turns a traffic spike into "too
  many connections" for every other consumer of that server.
- 2026-08-25 — Fixed a bug I introduced: the deferred `db.Close()` sat in `main`
  alongside `os.Exit(1)`, which skips defers. Extracted `run(logger) error` so
  cleanup actually happens on the error path.
- 2026-08-25 — ✅ All criteria complete. `database.go` + `database_test.go` added;
  3 tests cover no-DSN, unreachable-but-bounded, and malformed DSN.
- 2026-08-25 — Verification caveat: the **Docker daemon is not responding on this
  machine**, so the repo convention of running Go inside the `api` container could
  not be followed. Verified on the host instead with `GOTOOLCHAIN=auto` (resolves
  go1.25.8 per `go.mod`): `go build`, `go vet`, `go test ./...` and
  `go tool staticcheck ./...` all clean, `gofmt` reports nothing. **Not** verified:
  an actual connection to MariaDB, which needs the container stack.
- 2026-08-25 — Moving to done.
