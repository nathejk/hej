# 246 — Trailer registration in onboarding and on the profile page

**Status:** open
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

## Description

Let a user register a trailer. **A trailer counts as a second vehicle** (PRD 010 §11): its
own registration, its own row, `kind = trailer` — never a boolean or a note on the towing
car. That is what keeps the inventory countable and the pickup pool query honest.

- **Onboarding (task 241's step):** an explicit "tilføj anhænger" affordance after the car,
  not a separate flow — the two are brought together in practice, so they are gathered in
  one pass. But the confirmation must show **two entries**, so the user's mental model
  matches what the inventory holds.
- **A trailer with no car is legitimate** and must not be blocked (PRD 010 §5): someone may
  tow with a car registered by somebody else, or bring a trailer on its own.
- **A trailer has no seat count.** Do not ask for one. The pickup query is
  `kind = car AND seatCount > 0`, so a trailer cannot be dispatched even if a value slipped
  through, but asking would imply it could carry someone.
- **Profile (task 242's section):** trailers listed alongside cars, visually distinguished
  (Lucide `Truck` against `Car`), each independently editable and removable.
- The BFF endpoints accept and return `kind`, defaulting to `car` when absent so task 238's
  existing clients and tests keep working.

**Do not start this before task 245 is deployed.** Until `hq` filters dispatch to
`kind = car`, every trailer registered here appears in the coordinator's pickup list.

Depends on tasks 241, 242, 243, 244 and 245.

## Acceptance Criteria

- [ ] Task 245 confirmed deployed before this is picked up — logged, not assumed
- [ ] `kind` accepted and returned by the vehicle endpoints, defaulting to `car`
- [ ] A trailer is registered as a separate vehicle with its own id and plate
- [ ] A trailer without a car is accepted
- [ ] No seat count is asked for, or stored, for a trailer
- [ ] Onboarding gathers car and trailer in one pass but confirms two vehicles
- [ ] Profile lists trailers distinguishably, each independently editable and removable
- [ ] Tests: trailer-only registration, car + trailer producing two rows, and that a
      trailer is absent from any car-only read
- [ ] Backend and frontend suites, lint and type-check green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
