# 058 — Non-blocking broker startup and degraded-mode reads

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008 §5/§6. During an event, degraded and serving beats correct and dead. A
broker outage must not take the app down: reads come from SQL projections, so a
stale-but-present projection is far better than an unbootable API.

Startup must therefore not block on JetStream, and a broker that disappears at
runtime must not crash the process or wedge request handling. Reconnection should
be attempted in the background.

Note the deliberate asymmetry: the **database** IS on the login path, so a
database outage does break login (PRD 008 §5 accepts this). The broker is not.

## Acceptance Criteria

- [x] API starts and serves reads with the broker unreachable
- [x] Broker connection attempted asynchronously with retry/backoff
- [x] Loss of the broker at runtime is logged, not fatal
- [x] Publish attempts with no broker fail the request cleanly (a clear error,
      no panic, no silent success)
- [x] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Task 054 already made a missing broker non-fatal, but the
  connect was **synchronous**, so a slow broker still delayed startup and a broker
  that came up later was never picked up until a restart.
- 2026-08-25 — The case that actually matters is not "broker missing forever" (that
  already degraded fine) but "broker comes up thirty seconds after we do", which is
  the normal case when a whole compose stack starts at once. Without a retry the API
  would run publish-less indefinitely.
- 2026-08-25 — Design problem this created: the publisher now arrives *after*
  handlers are wired, so handing `commands.New` a publisher at boot no longer works.
  Introduced `commands.PublisherHolder` — an `atomic.Pointer` the background
  connector installs into and handlers read per request. `atomic.Pointer` rather than
  a mutex because the read is on every write request and the write happens once or
  twice per process lifetime.
- 2026-08-25 — Deliberate detail: the holder stores the interface *behind a pointer*
  so a typed-nil publisher cannot be mistaken for a live one. Clearing is explicit
  (`Set(nil)`), which keeps `Available()` honest.
- 2026-08-25 — Backoff is capped exponential (1s → 30s). The **first** failure logs at
  warn and the rest at debug: a broker down for an hour should not produce an hour of
  identical error lines that bury everything else in the log.
- 2026-08-25 — Projection registration and `writer.Arm()` moved into the connect
  callback, so they happen when there is actually something to consume from rather
  than at boot.
- 2026-08-25 — Verified with the **race detector** (`go test -race ./...`) since this
  introduces concurrency: clean. Also guarded `eventing`'s mutable fields with a
  mutex, as the background goroutine writes what the request path reads.
- 2026-08-25 — staticcheck flagged an `eventing.connected()` helper I had added for
  the healthcheck as unused. Removed it rather than shipping dead code or weakening
  the gate — task 059 adds it where it is actually consumed.
- 2026-08-25 — ✅ All criteria complete. 3 new tests: nil holder, empty holder, and
  publisher-arriving-late (the case the holder exists for). Green on the workspace
  and `GOWORK=off` paths, with `-race`, vet, staticcheck and gofmt.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so "broker
  appears late" was verified at the unit level (holder semantics), not by actually
  starting a broker after the API. The retry loop itself is not covered by a test —
  it needs a real endpoint to fail against.
- 2026-08-25 — Moving to done.
- 2026-08-26 — **Reopened and fixed: degraded-mode reads did not actually work.** Found
  while verifying PRD 008's success metrics before shipping the PRD, and the verification
  this task explicitly recorded as *not* done — running against a real unreachable endpoint
  rather than at the unit level — is exactly what exposed it.

  With `JETSTREAM_DSN` pointing at an unreachable host, the API served requests (200s) but
  the person projection was never constructed, because construction lived **inside** the
  broker's connect callback. The callback never ran, so `switchableDirectory` stayed on
  `users.NewMockDirectory()` while a fully populated `person` table sat in the database.

  Measured, not inferred:

  | with the broker unreachable | before | after |
  |---|---|---|
  | real member `+4520351385` requests a PIN | **no PIN** | PIN issued |
  | mock number `+4530000001` requests a PIN | **PIN issued** | no PIN |

  Both halves were wrong, and the second is the serious one: a broker outage in production
  silently turned the mock directory's phone numbers into working accounts with real roles,
  at exactly the moment nobody is watching closely. The first half contradicted this task's
  own promise and PRD 008 §5 — reads are supposed to come from existing projections, which
  need no broker at all.

  The fix follows the real dependencies rather than the old grouping: the projection is now
  constructed as soon as there is a **database**, and the broker decides only whether it
  keeps *updating*. The reasoning that had put it in the callback — "only create tables once
  there is a broker, so a database-only run does not create tables nothing will fill" — was
  a trivial saving weighed against a correctness and security bug.

  Also corrected a log line added in the same change: it reported `live=true` while the
  broker was demonstrably unreachable. Liveness is not knowable at that point (the
  connection is attempted in the background), so it now reports `broker_configured`, which
  is what is actually known.

  Verified in both directions: with the broker unreachable, real members log in and mock
  numbers do not; with it restored, `projections running, dead-letter queue empty` and 0
  dead letters. Gates green on the workspace and `GOWORK=off` paths.
