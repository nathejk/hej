# 234 — shared-go: filter vehicles by custodian

**Status:** doing
**Priority:** high
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:**

## Description

`GET /api/me/vehicles` (task 237) is custodian-scoped by definition, and
`vehicle.Filter` in `shared-go/tables/vehicle/filter.go` cannot express that: it narrows
by `YearSlug`, `SectionSlug`, `Unassigned` and `DriverUserIDs`.

Filtering by `DriverUserIDs` is not a substitute, and this is the reason the task exists
rather than a preference. The driver changes as the keys are handed on — that is what
`AssignDriver` is for — so a crew member who lent their car out for one pickup would
watch it vanish from "my vehicles", and the borrower would see it appear under theirs
with the edit and delete rights that task 239's authorisation rule deliberately does not
grant them. Custodianship is the stable fact, and it is the one the endpoint means.

Add `CustodianUserIDs []types.UserID` to `Filter`, mirroring `DriverUserIDs` exactly:
an empty slice does not filter, and it is not the same as asking for vehicles with no
custodian. Apply it in `querier.GetAll`.

This is in **shared-go**, not here — `hej` does not fork the entity (PRD 010 §8). In dev
`go/go.work` resolves the sibling checkout, so the change is usable immediately; the
version bump for `GOWORK=off` builds is task 244's job, shared with the trailer half.

PRD 010 §8, "Reading a caller's own vehicles".

## Acceptance Criteria

- [ ] `Filter.CustodianUserIDs []types.UserID` exists, documented in the same shape as
      `DriverUserIDs` including the empty-slice meaning
- [ ] `querier.GetAll` applies it, and combines with the other filter fields
- [ ] An empty slice does not filter
- [ ] A test covers: custodian match, several custodians, empty slice, and that a vehicle
      whose *driver* differs from its custodian is still returned to the custodian
- [ ] `go test ./...` green in shared-go

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up. Plan: mirror `DriverUserIDs` exactly in `filter.go` and
  `querier.GetAll`, then a test asserting the driver-differs-from-custodian case, which is
  the whole reason the field exists.
