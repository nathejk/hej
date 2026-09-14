# 239 — BFF: PATCH and DELETE /api/me/vehicles/{id}

**Status:** open
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `PATCH` and `DELETE /api/me/vehicles/:id` behind `requireAuth`
- [ ] Custodian-only; another custodian's vehicle answers `404`, never `403`
- [ ] A non-existent id and somebody else's id are indistinguishable in the response
- [ ] PATCH decodes into pointers: absent leaves, zero value clears
- [ ] PATCH of an unchanged form publishes nothing
- [ ] Changing a plate is duplicate-checked with the same code as task 238
- [ ] DELETE is idempotent
- [ ] A publish failure fails the request
- [ ] OpenAPI annotations present
- [ ] Handler tests for every criterion, including the absent-vs-zero distinction and the
      cross-custodian `404`
- [ ] `go test ./...`, `go vet`, `staticcheck` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
