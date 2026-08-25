# 055 — xstream.Mux and the projector registration pattern

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008 §8. Add an `xstream.Mux` that fans subjects to projectors, and establish
the three-way registration `hq` uses for every table entity:

1. construct it
2. add it to the `projections` slice registered on the mux
3. pass it into `data.NewModels(...)` and/or the write facade

There are no projectors yet (PRD 006 adds the first). This task lands the
pattern and an empty-but-wired mux, so adding one is mechanical.

## Acceptance Criteria

- [x] `xstream.Mux` constructed from the JetStream connection
- [x] A `projections []cqrs.Consumer` slice exists and is registered on the mux
- [x] The registration pattern is documented in a comment so the next entity
      follows it rather than inventing one
- [x] Empty projection set does not error at startup
- [x] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Completed as part of task 054's change. The mux is constructed by the
  same function that builds the publisher (it takes the same `js` handle), so
  splitting them across two commits would have left an intermediate state with a
  publisher and no way to consume — not independently useful or testable.
- 2026-08-25 — Implemented as `eventing.registerProjections(logger, projections...)`
  plus `eventing.run(ctx)`. The three-way registration is documented in the doc
  comment on `registerProjections`, including **why** the slice exists in one place:
  miss the registration step and the entity's tables simply never fill — reads work,
  return nothing, and nothing errors. That silence is the failure mode worth warning
  the next implementer about.
- 2026-08-25 — `mux.Run` is non-blocking (confirmed against `hq` and `tilmelding`,
  which both call it before serving HTTP), so it is called during startup and its
  error is logged rather than fatal — a broker problem must not stop the API serving
  reads.
- 2026-08-25 — Empty set verified by test: `TestEventingWithoutBrokerTolerated` and
  the nil-receiver test both register zero projections without error. When
  projections are registered but there is no broker, it warns rather than failing
  silently.
- 2026-08-25 — ✅ All criteria complete; see task 054's log for the verification
  matrix and caveats (no Docker daemon, so no real broker was exercised).
- 2026-08-25 — Moving to done.
