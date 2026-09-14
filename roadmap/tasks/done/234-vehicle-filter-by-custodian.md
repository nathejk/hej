# 234 — shared-go: filter vehicles by custodian

**Status:** done
**Priority:** high
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

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

- [x] `Filter.CustodianUserIDs []types.UserID` exists, documented in the same shape as
      `DriverUserIDs` including the empty-slice meaning
- [x] `querier.GetAll` applies it, and combines with the other filter fields
- [x] An empty slice does not filter
- [x] A test covers: custodian match, several custodians, empty slice, and that a vehicle
      whose *driver* differs from its custodian is still returned to the custodian
- [x] `go test ./...` green in shared-go

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up. Plan: mirror `DriverUserIDs` exactly in `filter.go` and
  `querier.GetAll`, then a test asserting the driver-differs-from-custodian case, which is
  the whole reason the field exists.
- 2026-09-14 — `Filter.CustodianUserIDs` added, mirroring `DriverUserIDs` including the
  empty-slice meaning, with the driver-vs-custodian reasoning written on the field rather
  than left in this task file — it is the kind of substitution that looks harmless at the
  call site.
- 2026-09-14 — Extracted `querier.allDataset(Filter)` from `GetAll`, following payment's
  `queries_test.go` pattern: the filter rules are then assertable without a database, which
  is the only way to pin down "which fields narrow" and "which override each other".
- 2026-09-14 — Two of my first assertions were wrong in a way worth recording: I asserted
  against the whole statement, and every column the filter touches is *also* in the SELECT
  list, so "the SQL mentions driverUserId" is true regardless of what the filter did. Added
  `whereOf` and asserted the predicate only. A test that cannot fail is worse than none.
- 2026-09-14 — Found while writing those tests: `GetAll` interpolated its filter values
  into the statement rather than using placeholders (`custodianUserId IN ('u1')`).
  Pre-existing, but this task is what puts a session-derived value through it, and a section
  slug or plate can plausibly arrive from a request. Marked the dataset `Prepared(true)` —
  matching what payment's tests say happened there — and added an injection-shaped test.
  `GetByID` was already parameterised via `QueryRowContext`.
- 2026-09-14 — ✅ Criterion 4, with one honest note: the driver-differs-from-custodian case
  is asserted **structurally** rather than against rows — the test proves a custodian filter
  emits no `driverUserId` predicate at all, so such a vehicle is not merely returned but
  cannot be excluded. That is a stronger guarantee than a fixture would give, and the
  querier has no database-backed test harness in this repo to write the fixture in.
- 2026-09-14 — ✅ All criteria. `go build ./...`, `go vet` and `go test ./...` green in
  shared-go. Also ran `go build` and the full `go test` suite for `hej` **inside the api
  container** (where `go.work` resolves the sibling checkout): green, so nothing downstream
  broke on the `GetAll` refactor.
- 2026-09-14 — shared-go committed as `e126b80`. **Not yet pushed or version-bumped** —
  that is task 244, shared with the trailer half. Until then `hej` compiles only with the
  workspace active, and the api container's `pinned_build` gate will say so.
