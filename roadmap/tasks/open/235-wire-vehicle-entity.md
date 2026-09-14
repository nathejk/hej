# 235 — Wire shared-go's vehicle entity into hej

**Status:** open
**Priority:** high
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

## Description

`hej` has no vehicle code at all today. Import shared-go's `tables/vehicle` and wire it
through PRD 008's three-way registration in `go/cmd/api/main.go`, the same way `person`
and `checkpoint` are wired:

1. construct it — `vehicle.New(ev.publisherOrNil(), ev.writer, ev.reader)`
2. add it to the consumer mux, so `NATHEJK.*.vehicle.*` projects locally
3. expose its read API on `data.Models`, and its `Commands` to handlers

Do **not** reimplement the entity, add a local table, or write SQL from a handler
(PRD 010 §8; `go-bff-layout`'s "Don'ts").

Two things to get right, both copied from how `person` and `checkpoint` already do it:

- **It may be nil.** Running without a database is a supported mode (PRD 008 §5), so
  `Models.Vehicles` follows `RaceAreas`/`People`: nil when there is no reader, and
  handlers check rather than assume. A nil vehicle read model means "vehicle data
  unavailable", which is a different answer from "you have no vehicles".
- **The write path is `vehicle.Commands`, not `commands.Commands.Publish`.** The entity
  owns its subject vocabulary and its dirty-checking (`UpdateFields.diff`), so publishing
  `NathejkVehicleRegistered` by hand from a handler would duplicate both. The existing
  `commands.Commands` facade stays as it is for the verification event; vehicles get the
  entity's interface, which is what `go-bff-layout` describes as the mature shape.

Blocked on nothing. Task 234 is only needed by task 237's read.

## Acceptance Criteria

- [ ] `vehicle.New` constructed in `main.go`, guarded on the database like `person` and
      `checkpoint` are
- [ ] Registered on the consumer mux, so vehicle events project
- [ ] The read API is reachable from handlers via `data.Models`, typed as
      `vehicle.Queries`, documented as nillable
- [ ] The `Commands` interface is reachable from handlers, and no handler can publish a
      vehicle event without it
- [ ] `go test ./...`, `go vet ./...` and `staticcheck ./...` green
- [ ] The `vehicle` table is created in the dev database on boot (verify in the container,
      not on the host)

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
