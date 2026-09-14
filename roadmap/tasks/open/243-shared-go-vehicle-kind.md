# 243 — shared-go: a kind on the vehicle entity (car / trailer)

**Status:** open
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

## Description

Trailers count as vehicles in their own right (PRD 010 §11, 2026-09-14): a trailer gets its
own registration and its own row, told apart from a car by a `kind`. shared-go's vehicle
entity has no such notion today — it documents itself as "one of the cars that collects
members off the route".

In **shared-go** (the entity's owner; `hej` does not fork it):

- `types.VehicleKind` with `car` and `trailer` plus a `Valid()` method, shaped like
  `types.MemberStatus`. A bare string field would put us back in PRD 006 §8's
  section-slug situation, where an unknown value is merely unexpected instead of
  detectable.
- `kind` on the `vehicle` table, `NOT NULL DEFAULT "car"`, added through
  `cqrs.EnsureColumn` for existing databases as well as `table.sql` for fresh ones.
- `Kind` on `RegisterFields`, on `NathejkVehicleRegistered`, and on the `Vehicle`
  projection struct. Add it to `vehicleColumns` and to `GetByID`'s explicit column list.
- `Filter.Kind`, so a caller can ask for cars only.

**The trap, and the reason this task is not just "add a column":** projections are rebuilt
by replaying the log, so every historical `vehicle.registered` event — all of which predate
this field — is re-applied on the next boot. The default must live in the **projector**, not
only in the column definition: a consumer that writes an empty `kind` will overwrite the
column default on every replay, and every existing car silently drops out of the pickup
pool (`kind = car AND seatCount > 0`). A SQL backfill would be undone by the next rebuild.

Naming: `kind` rather than `type`, because `type` is a Go keyword — the field could be
`Type` but no local variable can, which produces `typ`/`vType` at every call site.
Reversible now, awkward once the field ships (PRD 010 §12.6).

No `towedBy` relation (PRD 010 §11): knowing a trailer is present serves parking, insurance
and site safety; knowing which car pulls it serves nothing anyone has asked for, and an
unmaintained relation rots.

## Acceptance Criteria

- [ ] `types.VehicleKind` with `car`, `trailer` and `Valid()`
- [ ] `kind` column added in `table.sql` and via `EnsureColumn`, `NOT NULL DEFAULT "car"`
- [ ] `Kind` on `RegisterFields`, `NathejkVehicleRegistered` and the `Vehicle` projection
- [ ] `Filter.Kind` narrows reads
- [ ] The **projector** defaults an absent or empty kind to `car`
- [ ] A test replays a pre-`kind` registration event and asserts the row comes out as
      `car` — this is the criterion that protects the pickup pool
- [ ] An unknown kind on an event is detectable rather than silently stored
- [ ] `go test ./...` green in shared-go
- [ ] shared-go committed and pushed (task 244 does the bump here)

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
