# 238 — BFF: POST /api/me/vehicles

**Status:** done
**Priority:** high
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

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

- [x] `POST /api/me/vehicles` behind `requireAuth`, custodian taken from the session
- [x] Role gate written as "every role except spejder"; a spejder gets `403`
- [x] Plate normalised via `internal/plate` before validation, comparison and publish
- [x] Missing or implausible plate → `400`
- [x] Duplicate plate within the event year → `409`, without disclosing the other
      registrant's identity
- [x] A duplicate from a previous year is not a duplicate
- [x] Registration publishes through `vehicle.Commands.Register`; no hand-rolled event
- [x] A publish failure fails the request
- [x] OpenAPI annotations present
- [x] Handler tests for every criterion above
- [x] `go test ./...`, `go vet`, `staticcheck` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up.
- 2026-09-14 — The role gate is `users.MayRegisterVehicle` in `internal/users/vehicles.go`,
  a one-line mirror of `MayUseContacts`: `viewer.Valid() && viewer != RoleSpejder`. Put next
  to the existing access predicates rather than inlined in the handler so the exclusion is
  stated once, in the package that already owns "who may do what".
- 2026-09-14 — **Second shared-go gap, same shape as task 234's.** `Filter` could not narrow
  by plate either, so there was no way to ask "is this plate already registered" without
  reading the whole year's inventory into the handler — which would also mean the request
  path routinely handling every other member's custodian ids to answer a yes/no question.
  Added `Filter.LicensePlate` in shared-go (`fc58644`), pushed, and bumped `go.mod`.
- 2026-09-14 — While adding that filter's test, found the `whereOf` helper I wrote in task
  234 was too loose: it returned everything after `WHERE`, and the `ORDER BY licensePlate`
  tail meant a plate assertion would pass no matter what the filter did. Now cut at
  `ORDER BY`. Second time in this PRD that an assertion could have passed for the wrong
  reason — worth watching for in the remaining tasks.
- 2026-09-14 — A test of mine failed and taught me the code was stronger than I assumed:
  `app.ReadJSON` disallows unknown fields, so a `custodian_user_id` in the body is refused
  with `400` rather than silently ignored. Rewrote the test to assert that, and added a
  separate one pinning the published custodian to the session's user — the two halves of the
  guarantee task 239's authorisation depends on.
- 2026-09-14 — Deliberate decision on the duplicate check: it is **skipped, not fatal**, when
  the read model is unavailable. Refusing an otherwise valid registration because we cannot
  check for a duplicate would keep a real car out of the inventory in order to avoid a
  duplicate row — the worse of the two outcomes, since the inventory's purpose is knowing
  what is on site. Covered by `TestRegisterVehicleProceedsWithoutADuplicateCheck`.
- 2026-09-14 — The `409` names no registrant. A plate maps to a person, and this endpoint is
  not a lookup surface — asserted, not just commented.
- 2026-09-14 — ✅ All criteria. 11 tests for this endpoint, `go vet` and `staticcheck` clean,
  and the full suite green under `GOWORK=off` as well as with the workspace.
