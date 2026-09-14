# 237 — BFF: GET /api/me/vehicles

**Status:** open
**Priority:** high
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `GET /api/me/vehicles` registered in `routes.go` behind `requireAuth`, with a
      comment on why it is session-scoped
- [ ] Custodian-scoped via `Filter.CustodianUserIDs`, never by driver
- [ ] Empty list + `200` for a caller with no vehicles
- [ ] Unavailable read model is distinguishable from an empty list
- [ ] `401` without a session
- [ ] OpenAPI annotations present
- [ ] Handler tests: own vehicles only, empty case, unauthenticated, and one asserting a
      vehicle whose driver is somebody else is still returned to its custodian
- [ ] `go test ./...`, `go vet`, `staticcheck` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
