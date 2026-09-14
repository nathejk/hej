# 238 — BFF: POST /api/me/vehicles

**Status:** open
**Priority:** high
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

## Description

Register a vehicle, via `vehicle.Commands.Register`. The caller becomes the custodian —
taken from the session, never from the body — and the projector makes the custodian the
first driver, so nothing here needs to assign one.

Role gate: **every role except spejder** (PRD 010 §6). Write it as an exclusion, mirroring
`allRolesExcept('spejder')` on the client and `MayUseContacts`'s shape on the server, so a
role added to `AllRoles` later is included rather than silently refused. Spejdere get
`403`.

Body: plate (required), brand, model, colour, seat count, description. Normalise the plate
through `internal/plate` (task 236) before anything else — validation, duplicate detection
and storage must all see the same string.

**Duplicate plates.** Two people registering one car is the most likely data problem
(PRD 010 §5): a crew member and their passenger both fill it in. So a plate already
registered for the current year answers `409` rather than creating a second row. Two
things not to get wrong:

- The response must be usable by the frontend to say *"this car is already registered"* —
  but it must **not** leak who registered it beyond what the caller may see. A plate maps
  to a person, and this endpoint is not a lookup surface.
- Detection is per **event year**, not global. Last year's cars are not duplicates.

Seat count is the field most likely to be wrong in a way that matters — a coordinator
dispatching a car with one seat too few at 02:00 (PRD 010 §7). The API takes it as
"excluding the driver" because shared-go does; task 241 owns making that unmissable in the
UI.

A write that cannot be published must fail the request (PRD 008 §5, `ErrNoPublisher`), not
report success.

OpenAPI annotations mandatory: `201` / `400` / `401` / `403` / `409`.

Depends on tasks 235 and 236.

## Acceptance Criteria

- [ ] `POST /api/me/vehicles` behind `requireAuth`, custodian taken from the session
- [ ] Role gate written as "every role except spejder"; a spejder gets `403`
- [ ] Plate normalised via `internal/plate` before validation, comparison and publish
- [ ] Missing or implausible plate → `400`
- [ ] Duplicate plate within the event year → `409`, without disclosing the other
      registrant's identity
- [ ] A duplicate from a previous year is not a duplicate
- [ ] Registration publishes through `vehicle.Commands.Register`; no hand-rolled event
- [ ] A publish failure fails the request
- [ ] OpenAPI annotations present
- [ ] Handler tests for every criterion above
- [ ] `go test ./...`, `go vet`, `staticcheck` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
