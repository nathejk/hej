# 054 — Construct the cqrs triple in main.go

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008 §6/§8. Build the three interfaces every shared-go entity expects,
mirroring `hq`'s wiring:

| Interface | Role | Implementation |
|---|---|---|
| `cqrs.Reader` | query side | `*sql.DB` |
| `cqrs.Writer` | projection side | `deadletter` wrapping `sqlpersister` |
| `cqrs.Publisher` | command side | `metatagger` over JetStream |

Entity constructors take these interfaces — never a `*sql.DB` or a concrete
stream. Depends on tasks 050 (db) and 052 (deps).

## Acceptance Criteria

- [x] `cqrs.Reader`, `cqrs.Writer`, `cqrs.Publisher` constructed in `main.go`
- [x] JetStream connection established via `jrgensen/stream/jetstream`
- [x] No handler or package outside `main.go` receives a `*sql.DB` directly
- [x] Startup does not block on the broker (see task 058 for the full behaviour)
- [x] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Read the actual APIs in the module cache and the wiring in
  `hq` and `tilmelding` rather than guessing signatures.
- 2026-08-25 — Followed **`tilmelding`**, not `hq`, for the writer: `hq` uses
  `sqlpersister.New(db)` bare, while `tilmelding` wraps it
  (`deadletter.New(sqlpersister.New(db), db)`) — which is what PRD 008 §8 specifies
  and the better default, since one bad statement should not stall every projection
  behind it.
- 2026-08-25 — Learned the dead-letter lifecycle from `tilmelding` and reproduced it:
  create its table, `Reset()` stale entries from the previous run's replay, and only
  `Arm()` **after** projections are running. Arming late is the important part —
  before that point a failing statement means a broken schema and should crash the
  boot rather than be quietly captured.
- 2026-08-25 — Put the seam in its own `cmd/api/eventing.go` rather than inlining it
  in `main.go`, so `main` stays a wiring list. Publisher metadata uses shared-go's
  typed `messages.Metadata` with `Producer: "hej-api"` and the real build version,
  not `hq`'s untyped map or a copied producer name.
- 2026-08-25 — Two distinct degraded modes implemented, matching PRD 008 §5:
  **no database** → no eventing at all (nothing to project into); **no broker** →
  reader/writer still live, so handlers keep serving what the last run projected.
- 2026-08-25 — Bug found by my own test: `registerProjections` had no nil-receiver
  guard, so the no-database path would have crashed at startup — exactly the case the
  design claims to support. Added the guard and kept the test.
- 2026-08-25 — `GOWORK=off` build broke with missing `go.sum` entries for `nats.go`
  (the workspace was covering them via `go.work.sum`). Fixed with
  `GOWORK=off go mod tidy`, which also promoted shared-go/cqrs/stream from
  `// indirect` to direct — clearing the caveat left in task 052.
- 2026-08-25 — ✅ All criteria complete. Verified on **both** paths: workspace and
  `GOWORK=off` (the CI/prod path) — build, `go test ./...`, `go vet`, `staticcheck`,
  `gofmt` all clean.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so nothing was
  run against a real MariaDB or a real broker. Unit tests cover the nil/degraded
  paths only. **Not** verified: an actual connection, dead-letter table creation, or
  `mux.Run` subscribing.
- 2026-08-25 — Moving to done. Task 055 (mux + registration pattern) was completed in
  the same change — they are one coherent unit and splitting the commit would have
  left an unusable half.
