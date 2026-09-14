# 245 — hq: filter dispatch to kind = car (separate repo)

**Status:** done
**Priority:** high
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

## Description

**This task's work is in the `hq` repo, not here.** It is tracked on this board because
PRD 010 creates the need and because it is a **hard prerequisite** for task 246 — not a
follow-up.

`hq` already exposes organiser-facing CRUD over the vehicle projection and reads it for
dispatch. Once trailers can be registered (task 246), every trailer lands in that
projection, and any dispatch or pickup view that does not filter will offer the coordinator
a trailer as a car to send to a member waiting on the route. That is a regression in
somebody else's surface, caused by this feature, and it would show up at the worst moment
rather than in a test.

So the pickup pool must become `kind = car AND seatCount > 0` in `hq` **before**
registration is enabled here. Ship in the other order and PRD 010 makes the coordinator's
tools worse.

Also bump shared-go in `hq`, since the `kind` field arrives with task 243.

## Acceptance Criteria

- [x] shared-go bumped in `hq` to a version containing `kind`
- [x] Every dispatch/pickup view and query filters to `kind = car`
- [x] A trailer in the projection cannot be selected for a pickup — asserted by a test in
      `hq`, not verified by inspection
- [x] Organiser vehicle lists still show trailers (the inventory is the point); it is
      *dispatch* that filters
- [x] `hq`'s tests and lint green
- [ ] Confirmed **deployed** before task 246 is started — committed locally, not pushed or
      released; see the log

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Held off initially: `hq` had uncommitted work in `dispatch.go`,
  `dispatchtour.go` and `main.go` — the exact files this touches. Confirmed clean afterwards
  (that work landed as `1359164`), then picked up.
- 2026-09-14 — Found **three** call sites that answer "what can we send", not one, and they are
  worth listing because their severity is inverted from their visibility:
  1. `seatWarnings` (`dispatchtour.go`) summed a trailer's `SeatCount` into a tour's capacity.
     Nothing stops an operator typing a seat count on a trailer, and the effect was to
     **silence the overload warning altogether** — the failure is invisible until a car turns
     up for five people with room for four.
  2. `unitDriver` (`dispatch.go`) returned a trailer's `DriverUserID`. This is not hypothetical:
     shared-go's projector makes the custodian the first driver of *any* registered vehicle, so
     every trailer carries one — a parked trailer would have been reported as the person behind
     the wheel.
  3. The board's unit list showed trailers as dispatchable, and a car-plus-trailer additionally
     tripped the "more than one vehicle" misconfiguration flag (PRD 009 §6) — a warning about
     two *cars* in one unit, which firing on a normal car and trailer would teach the desk to
     ignore.
- 2026-09-14 — `organisation.go` deliberately keeps every kind, and now says so in a comment
  ending "do not make this consistent with dispatch". Its section-deletion guard counts trailers
  too: a trailer stranded on a deleted slug is exactly as invisible as a car.
- 2026-09-14 — `hq`'s `fakeVehicleQueries.GetAll` ignored its filter, so the new tests would
  have passed whether or not the handlers asked for cars. Made it honour `Filter.Kind` — and
  only that, since the handlers do their own section matching in Go and the existing tests rely
  on getting the whole list. Fifth instance of this pattern in this PRD.
- 2026-09-14 — **Verified all three tests by removing the filters and re-running.** All three
  fail, and the seat-warning one fails with `"warnings": []` — the exact silence described
  above. Restored, green again.
- 2026-09-14 — shared-go bumped to `5514e4c` in `hq`. `go vet`, the full `go test ./...` and a
  `GOWORK=off` build and test run are all green; the dispatch composable's 33 frontend tests
  still pass (the client needed no change — the board simply stops receiving trailers).
- 2026-09-14 — Pre-existing `staticcheck` findings in `hq`, untouched and unrelated:
  `patruljenumber/saga_test.go:95` (`owned` unused) and
  `spejderstatus/consumer_test.go:31` (`last` unused).
- 2026-09-14 — Committed in `hq` as `ea4f845`, in that repo's prose commit style rather than
  this board's `task()` convention.
- 2026-09-14 — **Left the deployment criterion unticked deliberately.** The commit is local: I
  have not pushed `hq` or released it, and "deployed" is not something I can verify from here.
  Task 246 is gated on this being *live*, not merely written — so it stays open.
