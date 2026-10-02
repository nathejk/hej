# PRD 028 — Trailer registration and one plate spelling across apps

**Status:** draft
**Author:** agent session (Claude Code)
**Created:** 2026-10-02
**Last updated:** 2026-10-02
**Approved:**
**Shipped:**
**Target users:** every role except spejder (`allRolesExcept('spejder')`), as in PRD 010; organisers in `hq` for the plate half

---

## 1. Summary

This PRD covers the work PRD 010 did not finish. Let people register a trailer as its own
vehicle, and make the inventory hold one spelling of each licence plate, whichever app
registered it. PRD 010 shipped the car half and was used at the 2026 race. This PRD
contains only what is left. For background, see PRD 010.

## 2. Problem & Motivation

- **What problem does this solve?** Two gaps from PRD 010:
  1. **Trailers can't be registered from the app.** shared-go's `kind` exists (task 243)
     and `hq` filters dispatch to cars (task 245), but `hej` has no way for a person to
     register a trailer. The site inventory is missing every trailer.
  2. **One car can appear under two plate spellings.** `hq` stores plates as typed
     (`EC16795`). `hej` normalises them (`DK+CA63640`, task 236). Because of this,
     duplicate detection doesn't work across the two apps, and `hq`'s plate search can miss
     cars that were registered in `hej`.
- **Why now?** The race has been held. Before the next season's registration opens is the
  cheapest time to fix both.
- **Evidence.** Task 248 (both spellings found in the dev database while verifying task
  247). Task 245's log says its deploy was never confirmed. PRD 010 §10/§12.

## 3. Goals

- A trailer can be registered as a second vehicle, alone or together with a car.
- Every plate in the inventory has one canonical spelling, whichever app wrote it.
- Duplicate detection works across `hq` and `hej`.

## 4. Non-Goals

- Anything PRD 010 already shipped (car registration, onboarding step, profile section).
- A `towedBy` relation between car and trailer (PRD 010 §11/§12.5).
- Vehicle position or tracking (PRD 010 §12.9; it would need its own PRD).
- Fixing projection truncation on rebuild (task 350). This is a related risk (§8), not in
  scope.

## 5. User Stories & Scenarios

- As a **bandit**, I want to add the trailer I'm towing so that it is on the site
  inventory.
- As a **crew member** whose car an organiser already entered in `hq`, I want to be told
  it's already registered, not end up with a second row.
- As an **organiser**, I want `hq`'s plate search to find a car whichever app registered it.

Edge cases: a trailer with no car (allowed). Car and trailer registered in one pass (two
rows, and the confirmation shows two entries). Foreign plates (don't require Danish format).
Rows that existed before normalisation (must read back normalised after a projection
rebuild).

## 6. Requirements

### Functional

- [ ] Task 245 is confirmed pushed and deployed in `hq` before trailer registration is
      enabled. The confirmation is logged, not assumed.
- [ ] A trailer is a **second vehicle**: its own row, plate, and edit/remove. It is
      registered with `kind = trailer`, never as a flag on the car.
- [ ] Registering "a car and a trailer" yields two vehicles in the inventory.
- [ ] A trailer without a car is accepted. A trailer is never asked for a seat count.
- [ ] The vehicle endpoints accept and return `kind`, defaulting to `car`.
- [ ] Onboarding and profile show trailers next to cars, each one visually distinct.
- [ ] Decide where plate normalisation lives (§11.1) and record the decision.
- [ ] One normalisation implementation, used by both `hq` and `hej`.
- [ ] Existing plates read back normalised, and stay that way after a projection rebuild.
- [ ] A plate registered in `hq` is detected as a duplicate in `hej`, and the other way
      round.

### Non-Functional

As PRD 010 §6: reuse shared-go, Danish copy, least data, users see only their own vehicles.

## 7. UX / UI Notes

After the car form, add an explicit "tilføj anhænger" affordance in onboarding (task 241's
step) and in "Mine køretøjer" (task 242). Use Lucide `Truck` for trailers and `Car` for
cars. The plate half has no UI change in `hej`. In `hq`, the only visible change is that
plates are shown normalised.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):** the vehicle step, the profile section, and the vehicles store
  gain `kind`.
- **BFF (Go):** the `/api/me/vehicles` handlers accept and return `kind`. The plate
  normalisation call site may move to shared-go (§11.1).
- **API endpoints:** no new endpoints. `POST`/`PATCH /api/me/vehicles` and
  `GET /api/me/vehicles` gain a `kind` field. **Their OpenAPI annotations must be updated
  to match**, per the README.
- **Data / storage:** no new table. If normalisation moves into shared-go's write path,
  existing rows are corrected where the event is applied (projector) or by correcting the
  events. A SQL backfill is undone by the next replay (the same trap as task 243).
- **Dependencies & risks:**
  - Cross-repo order: shared-go (if chosen), then bump in `hq` and `hej`, then deploy `hq`.
  - **Related risk, not in scope: task 350.** Projections never truncate on rebuild. A
    rebuild has been seen to leave 42 vehicle rows against 38 live ones. A plate
    migration that relies on replay is only as trustworthy as the rebuild, so task 350's
    status should be known before the migration is verified.

## 9. Success Metrics

- No vehicle in the inventory has a non-canonical plate after rebuild.
- No duplicate-plate rows across apps need manual reconciliation next season.
- Trailers on site appear in the inventory (checked against what is physically parked).

## 10. Rollout / Task Breakdown

Order: settle §11.1, then do 248, confirm 245 is deployed, then do 246. Tasks already exist
and were retargeted from PRD 010:

- [ ] 245 (done; deploy confirmation carried over): confirm `hq`'s `kind = car` filter is
      pushed and deployed
- [ ] 248: one plate spelling across both apps
- [ ] 246: trailer registration in onboarding and on the profile page

## 11. Open Questions

Carried over from PRD 010 §12 (numbers there in brackets):

1. **[Q8] Where does plate normalisation belong?** It could go in shared-go's write path
   (preferred, correct by construction), in `hq`'s handler, or be applied at comparison
   time only (rejected in task 248). The migration must also survive a projection rebuild.
2. **[Q1] Non-users' vehicles** (guardians, other adults with no account): app or
   organiser tool?
3. **[Q2] Is a plate required?** Or should a provisional registration be allowed for a
   borrowed car or trailer whose plate is not known yet?
4. **[Q3] What else does the inventory feed?** Parking, access, insurance: are any fields
   missing (arrival/departure, insurance reference)?
5. **[Q4] Should trailers be self-registered at all**, or entered by organisers as
   equipment?
6. **[Q9] Vehicle position:** a possible separate PRD. Listed here only so it isn't lost.
