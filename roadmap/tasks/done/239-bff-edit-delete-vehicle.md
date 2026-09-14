# 239 — BFF: PATCH and DELETE /api/me/vehicles/{id}

**Status:** done
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

## Description

Editing and removing a vehicle the caller registered. Plans change (PRD 010 §3), and a
registration nobody can correct becomes a registration nobody trusts.

**Authorisation is the whole risk in this task.** Unlike the rest of `/api/me/*`, these
two carry an id in the path, so they are the first vehicle endpoints that *can* be pointed
at somebody else's row. A caller may edit or delete only vehicles they are the
**custodian** of — not the driver (task 234's reasoning applies again: a borrower must not
be able to delete the car they were lent). A vehicle that exists but belongs to somebody
else answers `404`, not `403`, so the endpoint cannot be used to discover which plates
exist — the same rule the patrol lookup follows in `patrol.go`.

**PATCH is a delta**, matching `vehicle.UpdateFields`: a field absent from the body leaves
the value alone, and a field present with a zero value clears it. That distinction has to
survive JSON decoding, so the request struct needs pointers — decoding into value types
would make "clear the description" and "do not touch the description" the same request.
The entity already prunes a delta to what actually changed, so re-saving an unchanged form
publishes nothing; do not re-implement that here.

Changing a plate re-opens the duplicate question from task 238 — reuse the same check
rather than writing a second one.

DELETE publishes `vehicle.deleted`, a soft delete in the read model. Idempotent: deleting
an already-deleted vehicle is not an error worth surfacing to someone whose car is, in
fact, not in the inventory.

OpenAPI annotations mandatory: `200`/`204` / `400` / `401` / `404` / `409`.

Depends on tasks 235, 236 and 238.

## Acceptance Criteria

- [x] `PATCH` and `DELETE /api/me/vehicles/:id` behind `requireAuth`
- [x] Custodian-only; another custodian's vehicle answers `404`, never `403`
- [x] A non-existent id and somebody else's id are indistinguishable in the response
- [x] PATCH decodes into pointers: absent leaves, zero value clears
- [x] PATCH of an unchanged form publishes nothing
- [x] Changing a plate is duplicate-checked with the same code as task 238
- [x] DELETE is idempotent — **in effect rather than in status code**; see the log
- [x] A publish failure fails the request
- [x] OpenAPI annotations present
- [x] Handler tests for every criterion, including the absent-vs-zero distinction and the
      cross-custodian `404`
- [x] `go test ./...`, `go vet`, `staticcheck` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up. Both handlers share an `ownVehicle` helper that resolves the path
  id *and* authorises it, so no handler can read one without the other — the authorisation is
  not a step a future handler can forget to call.
- 2026-09-14 — Extracted `plateTaken`, now used by both registration and a plate change. One
  implementation, because two would drift and the one that drifts is the one that stops
  catching duplicates. The check runs only when the plate actually differs, so re-submitting
  an unchanged form cannot report a vehicle as a duplicate of itself.
- 2026-09-14 — **Deviation from the criterion as written: DELETE is idempotent in effect, not
  in status code.** A second delete answers `404`. This is forced rather than chosen: a
  soft-deleted row is invisible to the read API, so "already deleted" is indistinguishable
  from "never existed" and "belongs to somebody else" — and telling those apart is exactly
  what must not be possible, or the endpoint becomes a way to discover which registrations
  exist. So what is guaranteed is that a repeat publishes no second event and the car stays
  gone, which is the property that actually matters. Criterion amended above rather than
  quietly marked done.
- 2026-09-14 — Fixed a real weakness in my own test suite. `fakeVehicles.GetAll` ignored the
  filter and returned everything, which made the plate-duplicate tests pass whether or not
  the handler narrowed by plate at all. The fake now honours the plate, custodian and driver
  filters. That is the third time in this PRD a test could have passed for the wrong reason
  (see tasks 234 and 238) — all three were fakes or helpers that were more permissive than
  the thing they stood in for.
- 2026-09-14 — The cross-custodian tests are written from the *driver's* side on purpose: the
  fixture is a car lent to the caller, who is driving it. Those are the cases where a
  plausible-looking implementation grants access, and where the cost is highest — a borrower
  deleting a car takes it out of the dispatch pool and its custodian never learns why.
- 2026-09-14 — ✅ All criteria. 14 tests for these two endpoints, `go vet` and `staticcheck`
  clean, full suite green under `GOWORK=off`.
