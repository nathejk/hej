# 245 — hq: filter dispatch to kind = car (separate repo)

**Status:** open
**Priority:** high
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] shared-go bumped in `hq` to a version containing `kind`
- [ ] Every dispatch/pickup view and query filters to `kind = car`
- [ ] A trailer in the projection cannot be selected for a pickup — asserted by a test in
      `hq`, not verified by inspection
- [ ] Organiser vehicle lists still show trailers (the inventory is the point); it is
      *dispatch* that filters
- [ ] `hq`'s tests and lint green
- [ ] Confirmed deployed before task 246 is started

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
