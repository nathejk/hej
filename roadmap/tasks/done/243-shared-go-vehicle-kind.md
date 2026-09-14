# 243 — shared-go: a kind on the vehicle entity (car / trailer)

**Status:** done
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

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

- [x] `types.VehicleKind` with `car`, `trailer` and `Valid()`
- [x] `kind` column added in `table.sql` and via `EnsureColumn`, `NOT NULL DEFAULT "car"`
- [x] `Kind` on `RegisterFields`, `NathejkVehicleRegistered` and the `Vehicle` projection
- [x] `Filter.Kind` narrows reads
- [x] The **projector** defaults an absent or empty kind to `car`
- [x] A test replays a pre-`kind` registration event and asserts the row comes out as
      `car` — this is the criterion that protects the pickup pool
- [x] An unknown kind on an event is detectable rather than silently stored
- [x] `go test ./...` green in shared-go
- [x] shared-go committed and pushed

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — `types.VehicleKind` in `types/vehicle.go`, shaped like `MemberStatus`. The empty
  string is deliberately **not** valid: an event that omits the field is defaulted at the
  projector, so a kind that reaches a caller empty is a fault rather than "unspecified".
- 2026-09-14 — Added `OrCar()` so the default exists in exactly one expression, and gave it its
  own test asserting it does **not** launder an unknown value into a car — defaulting and
  validating are different jobs, and an unknown kind has to survive to where it can be rejected
  with an error.
- 2026-09-14 — Column added in both `table.sql` *and* `cqrs.EnsureColumn`. `CREATE TABLE IF NOT
  EXISTS` is a no-op on every database that already has a vehicle table, so a column declared
  only in the schema file would be missing everywhere real and every projection statement naming
  it would dead-letter on an unknown column. Same reasoning payment's table already documents.
- 2026-09-14 — **Verified the replay guard by breaking it.** Changed the projector back to
  `body.Kind` and re-ran: `TestReplayOfAPreKindEventProducesACar` fails, and the failure output
  shows the INSERT writing `kind` as `''` — exactly the statement that would have emptied the
  pickup pool at the next rebuild. Then restored it. Worth the two minutes: this is the fourth
  time in this PRD that a test needed checking against the bug it claims to catch, and the first
  three were all passing for the wrong reason.
- 2026-09-14 — The description field's comment now says outright that a trailer does **not**
  belong in it: free text cannot be queried, so a trailer recorded that way is invisible to any
  count of what is on site — which is the whole point of the change.
- 2026-09-14 — ✅ All criteria. shared-go pushed as `5514e4c`; 9 new tests across `types` and
  `tables/vehicle`; `go test ./...` and `go vet` green.
