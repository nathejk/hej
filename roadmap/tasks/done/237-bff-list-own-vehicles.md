# 237 — BFF: GET /api/me/vehicles

**Status:** done
**Priority:** high
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

## Description

The caller's own vehicles, behind `requireAuth`, in `go/cmd/api/vehicles.go`.

Session-scoped by construction, like `/api/me/profile` and `/api/me/photo`: there is no
user id in the path, so no caller can ask for somebody else's. The filter is
`CustodianUserIDs: [caller]` (task 234) plus the current year — **not** `DriverUserIDs`,
for the reason task 234 spells out.

Details:

- A caller with no vehicles gets `200` and an empty list, not `404`. "You have none" is a
  successful answer and the frontend renders an empty state from it.
- A nil vehicle read model (no database, PRD 008 §5) is a `503`-class answer, not an empty
  list. Reporting "you have no vehicles" when the truth is "we cannot tell" would invite
  someone to register a second row for a car that is already there.
- OpenAPI annotations are mandatory (`.rules`): `200` / `401`.

Depends on tasks 234 and 235.

## Acceptance Criteria

- [x] `GET /api/me/vehicles` registered in `routes.go` behind `requireAuth`, with a
      comment on why it is session-scoped
- [x] Custodian-scoped via `Filter.CustodianUserIDs`, never by driver
- [x] Empty list + `200` for a caller with no vehicles
- [x] Unavailable read model is distinguishable from an empty list
- [x] `401` without a session
- [x] OpenAPI annotations present
- [x] Handler tests: own vehicles only, empty case, unauthenticated, and one asserting a
      vehicle whose driver is somebody else is still returned to its custodian
- [x] `go test ./...`, `go vet`, `staticcheck` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up. Plan: follow `raceAreaHandler` — it is the closest precedent for the
  unavailable-vs-empty distinction this endpoint also has to make.
- 2026-09-14 — Handler, `vehicleResponse` and route added. The response is a **projection**
  of `vehicle.Vehicle`, not the row: `custodianUserId`, `driverUserId` and `sectionSlug` are
  the coordinator's half of the entity and none of them is something an owner needs in order
  to see their own registration. Projecting server-side rather than trusting the client is
  the rule `.rules` states for guardian numbers, applied to a much smaller case — a response
  that does not carry a field cannot leak it however the client changes.
- 2026-09-14 — The list is wrapped in an object and built with `make(..., 0, n)`, so the
  JSON is `{"vehicles":[]}` rather than `null`. "No vehicles" then needs no special case on
  any surface that reads it.
- 2026-09-14 — Tests assert the **filter**, not just the JSON, because the custodian-vs-driver
  distinction is invisible in a response and is what decides whether a borrower can later
  delete somebody else's car (task 239). `TestOwnVehiclesAsksByCustodianNotByDriver` fails if
  `DriverUserIDs` is ever populated here.
- 2026-09-14 — ✅ All criteria: 7 tests, plus `go vet` and `staticcheck` clean, and the full
  suite green.
- 2026-09-14 — **Did task 244's bump early, deliberately.** This is the first code that uses
  `Filter.CustodianUserIDs`, so committing it would have left `main` building only with the
  workspace active — which `go-bff-layout` names outright as a broken build, and which the
  container's `pinned_build` gate immediately confirmed:
  `cmd/api/vehicles.go:127: unknown field CustodianUserIDs`. Pushed shared-go (`e126b80`) and
  bumped `go.mod` to `v0.0.0-20260914180220-e126b80fd5fa`. `GOWORK=off` build and test now
  pass, and the dev loop restarts cleanly (`projections registered count:3`). Task 244 is
  amended to cover only the `kind` bump.
