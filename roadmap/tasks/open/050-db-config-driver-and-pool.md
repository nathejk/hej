# 050 — DB config, MariaDB driver and pooled connection

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `DB_DSN` read in `main.go` into the existing `config` struct
- [ ] `github.com/go-sql-driver/mysql` added to `go.mod`
- [ ] Pooled `*sql.DB` with explicit `SetMaxOpenConns` / `SetMaxIdleConns` /
      `SetConnMaxLifetime`
- [ ] Startup ping with bounded retry and a clear log line on failure
- [ ] Connection closed on shutdown
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
