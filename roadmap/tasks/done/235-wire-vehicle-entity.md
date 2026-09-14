# 235 — Wire shared-go's vehicle entity into hej

**Status:** done
**Priority:** high
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

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

- [x] `vehicle.New` constructed in `main.go`, guarded on the database like `person` and
      `checkpoint` are
- [x] Registered on the consumer mux, so vehicle events project
- [x] The read API is reachable from handlers via `data.Models`, typed as
      `vehicle.Queries`, documented as nillable
- [x] The `Commands` interface is reachable from handlers, and no handler can publish a
      vehicle event without it
- [x] `go test ./...`, `go vet ./...` and `staticcheck ./...` green
- [x] The `vehicle` table is created in the dev database on boot (verify in the container,
      not on the host)

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up. Plan: follow `checkpoint`'s wiring in `main.go` exactly — it is
  the smaller of the two precedents — then add nillable `Vehicles` to `data.Models` and the
  entity's `Commands` to the application struct.
- 2026-09-14 — Hit a shape difference worth recording: `vehicle.New` returns an
  **unexported** `*table`, which is the house style across every shared-go entity, so the
  `var x *person.Table` pattern `main.go` uses for its two local projections does not
  compile here. Declared a local `vehicleTable` interface in `cmd/api/vehicles.go` naming
  the three roles the value fills (`Queries`, `Commands`, `cqrs.Consumer`). That is
  arguably the more honest expression anyway — `person` and `checkpoint` are concrete only
  because they happen to export their type.
- 2026-09-14 — Second difference: `vehicle.New` returns no error. It logs a failed
  CREATE TABLE internally and hands back a usable value, so unlike `person.New` and
  `checkpoint.New` there is no error branch to write — and no signal this process can act
  on. Noted in a comment at the construction site rather than worked around.
- 2026-09-14 — `vehiclesOrNil` / `vehicleCommandsOrNil` take the interface and return nil
  *for the interface*, rather than assigning the pointer directly. This is the nil-interface
  trap: a typed nil in an interface field is not `== nil`, so a handler's availability check
  would pass and then panic. Written as functions so that mistake is untypeable here.
- 2026-09-14 — Write side wired as `application.vehicles` (`vehicle.Commands`), separate
  from the existing `commands.Commands` publisher facade. Per `go-bff-layout` that is the
  mature shape, and it matters concretely: the entity owns its subject vocabulary and its
  delta pruning (`UpdateFields.diff`), both of which a handler reaching for the generic
  publisher would re-implement.
- 2026-09-14 — `data.NewModels` gained a fifth parameter, so twelve test call sites were
  updated to pass `nil`. Mechanical, but it is most of this task's diff.
- 2026-09-14 — `go.mod` picked up `goqu` as an indirect dependency: it comes in with the
  vehicle package's query side. Expected, not a new direct dependency.
- 2026-09-14 — ✅ All criteria. `go vet`, `staticcheck` and `go test ./...` green in the api
  container. Verified in the **running** dev stack rather than inferred: the boot log reports
  `projections registered count:3` (was 2), and `DESCRIBE vehicle` in the `hej` database
  shows the twelve columns from shared-go's `table.sql`.
- 2026-09-14 — Note for task 244: the container's `pinned_build` gate **passed** here, which
  is correct and not a contradiction of task 234's log — nothing in this task touches
  `Filter.CustodianUserIDs`, so the pinned shared-go version still satisfies it. Task 237 is
  the first code that will need the bump.
