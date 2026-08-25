# PRD 010 — Vehicle registration (cars and trailers, self-registered)

**Status:** draft
**Author:** agent session (Zed)
**Created:** 2026-08-25
**Last updated:** 2026-08-25
**Approved:**
**Shipped:**
**Target users:** bandit, gøgler and crew (every role that may bring a vehicle). Explicitly **not** spejder.

---

## 1. Summary

Let the people who bring vehicles to Nathejk register them from the app: a car,
and a trailer if they have one. Registration appears as a step in onboarding
(PRD 005) and remains editable on the profile page (PRD 003), so the organisers have
a complete inventory of every vehicle associated with the race without collecting it
by hand.

## 2. Problem & Motivation

- **What problem does this solve?** Every vehicle connected to the race needs to be
  known: for site parking and access, for insurance and liability, and \u2014 for cars \u2014
  because they are the pool that collects members off the route. Today that
  inventory is assembled by organisers rather than by the people who actually know
  the registration plate, and nothing in the participant-facing app asks.
- **Why the car half matters operationally.** `types.VehicleID` describes itself as
  "one of the cars that collects members off the route", and shared-go's
  `RegisterFields.SeatCount` documents "excludes the driver. Zero for a car brought
  only for its owner's own transport, which is not offered for pickups". So the
  vehicle inventory *is* the car pool behind `MemberStatusTransit` ("the member is in
  one of our cars"). A car nobody registered is a car the coordinator cannot dispatch
  when a member needs collecting at 02:00.
- **Why now?** PRD 006 gives the app real identities and roles, so it can finally
  tell who may register a vehicle. PRD 008 gives it a write path. Both prerequisites
  just landed.
- **Evidence.** `shared-go/tables/vehicle` already exists and is complete for cars:
  a `Commands` interface (`Register`, `Update`, `AssignDriver`, `AssignSection`,
  `Delete`), a projection, and five subjects under `NATHEJK.*.vehicle.*`. `hq` already
  exposes organiser-facing CRUD over it. What is missing is the participant-facing
  half and any notion of a trailer.

## 3. Goals

- A person who brings a vehicle can register it themselves, in under a minute, from
  the phone they already have.
- The organisers get one complete inventory of vehicles associated with the race.
- A car's seat count is captured, so the pickup pool is usable for dispatch.
- Registration is editable and removable afterwards \u2014 plans change.
- Reuse shared-go's existing vehicle entity rather than inventing a parallel one.

## 4. Non-Goals

- **Dispatching vehicles / assigning pickups.** That is `hq`'s coordinator surface,
  which already reads this projection. This PRD only puts vehicles *into* it.
- **Driver assignment beyond the registrant.** shared-go's `Register` makes the
  custodian the first driver, which is right for self-registration. Multi-driver
  management is an organiser concern (`AssignDriver` exists for it).
- **Section assignment.** `AssignSection` exists and is organiser-facing; a
  self-registering crew member's section is already known from PRD 006.
- **Spejder registering vehicles.** They are minors and do not drive to the event as
  drivers. The step must not appear for them.
- **Parking allocation, permits, or access control.** Downstream uses of the
  inventory, not this PRD.
- **Vehicle tracking / position.** Out of scope, and deliberately so \u2014 see §11.

## 5. User Stories & Scenarios

- As a **crew member**, I want to register the car I am bringing so the coordinator
  knows it exists and how many people it seats.
- As a **bandit**, I want to add the trailer I am towing so it is on the site
  inventory and nobody treats it as an unknown vehicle.
- As someone whose **plans changed**, I want to correct or remove a vehicle I
  registered.
- As an **organiser**, I want to see every vehicle without chasing people, and to be
  able to tell a car that can collect members from one that cannot.

### Primary path

1. During onboarding (PRD 005), a bandit/gøgler/crew member reaches the vehicle step.
2. They answer "are you bringing a vehicle?" \u2014 skipping is one tap.
3. If yes: registration plate (required), plus brand, model, colour, and **seats
   excluding the driver**.
4. Optionally "…and a trailer", which asks for the trailer's plate and a description.
5. Submitting publishes `vehicle.registered`, and the vehicle appears on the profile
   page from then on, editable and removable.

### Edge cases

- **Two people register the same car.** Realistic: a crew member and their passenger
  both fill it in. The plate is the natural identity, so a duplicate plate within a
  year should be detected and offered as "this car is already registered by X" rather
  than silently creating two rows the coordinator has to reconcile.
- **A trailer with no car.** Legitimate \u2014 someone may tow with a car registered by
  someone else, or bring a trailer separately. Must not be blocked.
- **Plate formatting.** shared-go asks for a country prefix (`"DK+AB12345"`). Users
  will type `ab 12 345`. Normalise, and do it in one place, or the duplicate
  detection above cannot work (the same lesson as phone numbers in PRD 006 §2).
- **A foreign plate.** Nathejk draws Danish participants but not exclusively; do not
  hard-require a Danish format.
- **Skipped then needed.** Someone who skips at onboarding must be able to register
  later from the profile page, without a nag that implies they did something wrong.
- **Offline.** Registration is a write, so it needs connectivity. It must fail
  clearly and be retryable, not silently drop (PRD 008: a write that could not be
  published has not happened).

## 6. Requirements

### Functional

- [ ] A vehicle step in onboarding for **bandit, gøgler and crew**; absent for
      spejder.
- [ ] Skipping is a single tap and carries no penalty.
- [ ] Registration captures: plate (required), brand, model, colour, seat count
      excluding the driver, free-text description.
- [ ] A **trailer** can be registered, optionally alongside a car, distinguished by
      the vehicle's `kind` rather than by a convention (§8).
- [ ] A trailer is never offered as a pickup vehicle: the pool is
      `kind = car AND seatCount > 0`.
- [ ] Plates are normalised in one place, shared by registration and duplicate
      detection.
- [ ] A duplicate plate within the event year is surfaced to the user rather than
      creating a second row.
- [ ] The profile page (PRD 003) lists the user's vehicles and allows edit and
      remove.
- [ ] Writes go through shared-go's `vehicle.Commands` \u2014 no new event vocabulary,
      no direct SQL.
- [ ] The registrant becomes the vehicle's custodian and first driver.
- [ ] A failed write is reported and retryable.

### Non-Functional

- **Reuse over reinvention.** shared-go owns the vehicle entity; this app is a
  client of it. Any new field belongs there, not in a parallel table here.
- **Danish copy**, per the other user-facing surfaces.
- **Least data.** Collect what dispatch and site safety actually need. A plate plus
  seats is operationally useful; anything more should justify itself.
- **Privacy.** A registration plate identifies a person's vehicle and, indirectly,
  them. It is not portrait-grade sensitive, but it is not public either: the app must
  show a user their own vehicles, and no one else's.

## 7. UX / UI Notes

- **Onboarding step** (PRD 005): a yes/no gate first, so the majority who bring
  nothing answer in one tap and never see a form.
- **The form** is short and ordered by what people know without looking: plate,
  then brand/model/colour, then seats. Seat count needs a label that makes
  "excluding the driver" unmissable \u2014 an off-by-one here means a coordinator
  dispatches a car with one seat too few at the worst moment.
- **Trailer** is an explicit "tilføj anhænger" affordance after the car, not a
  separate flow, since the two are registered together in practice.
- **Profile page** (PRD 003): a "Mine køretøjer" section listing each vehicle with
  plate and a summary line, plus edit/remove and an "add" action for anyone who
  skipped.
- shadcn-vue primitives and Lucide icons per `.rules` (`Car`, `Truck`, `Plus`,
  `Trash2`); `font-nathejk` for headings only.

## 8. Technical Considerations

### Trailers: a `kind` on the vehicle entity (decided 2026-08-25)

**shared-go's vehicle entity has no notion of a trailer**, and that is not an oversight
to route around. The type documents itself as "one of the cars that collects members
off the route", and `SeatCount` exists to say whether a car can be used for pickups. A
trailer is neither a car nor something that collects anyone.

So the request ("all vehicles that can be associated with the race need to be
registered") asks the inventory to serve a **second purpose** — site/safety/insurance
inventory — alongside the one it was built for.

**Decision: add a kind/type property to shared-go's vehicle entity.** One inventory
answers both "every vehicle on site" and "cars that can collect a member", and the
difference between them becomes a modelled fact rather than a convention.

The alternative was registering trailers as vehicles with `seatCount = 0`. Rejected:
that value **already** means "a car brought only for its owner's own transport, which is
not offered for pickups", so two unrelated facts would share one encoding — and every
trailer would land in the pickup pool where a coordinator could dispatch one.

#### What the change involves

In **shared-go** (it owns the entity; nothing here forks it):

- A `types.VehicleKind` string type with `car` and `trailer`, plus a `Valid()` method —
  the same shape as `types.MemberStatus`, so an unknown value is detectable rather than
  merely unexpected. A bare string field would put us back in the section-slug
  situation from PRD 006 §8.
- `kind` on the `vehicle` table, `NOT NULL DEFAULT "car"`.
- `Kind` on `RegisterFields`, and on the `NathejkVehicleRegistered` message.
- The projector defaults a missing `kind` to `car`.

That last point is the one that matters and is easy to miss: **projections are rebuilt
by replaying the log**, so every historical `vehicle.registered` event — all of which
predate this field — is re-applied on the next boot. Defaulting in the *projector*, not
only in the column, is what makes the replay produce cars rather than blanks. A SQL
backfill would be undone by the next rebuild.

In **`hq`** (separate repo, and required rather than optional):

- Dispatch and pickup views must filter to `kind = car`. Until they do, adding trailers
  to the inventory actively degrades the coordinator's surface — which is why the
  filtering task is listed in §10 as a prerequisite for shipping trailer registration,
  not as a follow-up.

Remember the two-repo release loop from `go-bff-layout`: shared-go must be committed,
pushed and version-bumped in both consumers before a `GOWORK=off` build sees the field.

#### Naming, and what stays out

- **`kind` rather than `type`.** `type` has precedent in this codebase
  (`types.TeamType`, the section message's `Type`), so either is defensible, but `type`
  is a Go keyword: the field can be `Type` while a local variable cannot, which produces
  `typ`/`vType` spellings at every call site. Trivially reversible if the team prefers
  consistency over ergonomics.
- **No `towedBy` relation.** I proposed one earlier; on reflection it is not needed for
  what this PRD is for. Knowing *that* a trailer is present serves parking, insurance and
  site safety; knowing *which car pulls it* serves nothing anyone has asked for, and a
  relation nobody maintains rots. Left as §12.
- **Seat count does not apply to a trailer.** The pickup query is
  `kind = car AND seatCount > 0`, so a trailer cannot be dispatched even if a seat count
  is somehow set.

### Where the code goes

- **Reuse, do not reimplement.** `hej` imports `shared-go/tables/vehicle`, registers
  its projection on the mux (PRD 008's three-way registration), and calls the existing
  `Commands` interface. No new subjects, no new table, no parallel notion of a vehicle.
- **This is `hej`'s first user-facing domain write**, after PRD 005's verification
  event. The write facade and publisher already exist (PRD 008 tasks 056/058), and the
  same rule applies: a write that cannot be published must fail the request, not report
  success.
- **`hq` already consumes these events**, so a vehicle registered in the app appears in
  the coordinator's view with no integration work. That is the payoff of using the
  shared entity.

### API endpoints (OpenAPI annotations mandatory, per `.rules`)

- `GET /api/me/vehicles` \u2014 the caller's own vehicles. `200` / `401`.
- `POST /api/me/vehicles` \u2014 register. `201` / `400` / `401` / `409` (duplicate plate).
- `PATCH /api/me/vehicles/{id}` \u2014 edit; delta semantics matching
  `vehicle.UpdateFields`, where nil leaves a field alone and a zero value clears it.
- `DELETE /api/me/vehicles/{id}` \u2014 remove (publishes `vehicle.deleted`).

Authorisation is per-caller: a user may only read or modify vehicles they are the
custodian of. Organiser-wide access stays in `hq`.

### Data / storage

No new table in `hej`. The `vehicle` projection is shared-go's, registered here so the
app can read its own rows without calling another service (PRD 008: services do not
call each other's APIs).

### Dependencies & risks

- **The trailer half depends on a shared-go change** (§8) landing and being
  version-bumped in both `hej` and `hq`. The car half needs none of it and ships first.
- **`hq` must filter dispatch to `kind = car` before trailer registration is enabled.**
  Shipping in the other order would put trailers in the coordinator's pickup list — a
  regression in someone else's surface, caused by this feature.
- **Depends on PRD 006** for roles: the step must appear for bandit/gøgler/crew and not
  spejder, which requires the real directory.
- **Risk: the replay defaults.** Every historical `vehicle.registered` event predates
  `kind`, and projections rebuild from the log on every boot. If the default lives only
  in the column and not in the projector, a rebuild produces blank kinds and every
  existing car silently drops out of the pickup pool.
- **Risk: duplicate registrations.** Two people registering one car is the most likely
  data problem, and plate normalisation is what makes it detectable.
- **Risk: seat-count misunderstanding.** "Excluding the driver" is easy to get wrong
  and directly affects dispatch.
- **Risk: scope creep towards tracking.** A registered vehicle plus a position feed is
  a tracking system. Kept out of scope deliberately; see §12.

## 9. Success Metrics

- Every vehicle on site at the event is in the inventory, verified by a spot check
  against what is physically parked.
- Coordinators dispatch pickups from the app's inventory rather than a hand-kept list.
- Zero duplicate-plate rows requiring manual reconciliation.
- ≥ 80% of bandit/gøgler/crew who bring a vehicle register it during onboarding rather
  than being chased.

## 10. Rollout / Task Breakdown

Sequence the car half first — it needs no shared-go change and delivers the
operational value. The trailer half follows, and its ordering across repos is not
negotiable: **shared-go, then `hq`'s dispatch filter, then trailer registration here.**
Enabling registration before `hq` filters would put trailers in the coordinator's
pickup list.

Proposed tasks for `roadmap/tasks/open/`:

**Car half (no cross-repo dependency):**

- [ ] Task: register shared-go's `vehicle` projection in `hej` (mux + models)
- [ ] Task: plate normalisation helper + tests (one implementation, shared)
- [ ] Task: BFF — `GET /api/me/vehicles` behind `requireAuth`, custodian-scoped
- [ ] Task: BFF — `POST /api/me/vehicles` via `vehicle.Commands.Register`, incl. duplicate-plate detection
- [ ] Task: BFF — `PATCH` / `DELETE` with custodian authorisation
- [ ] Task: onboarding vehicle step (yes/no gate, then form), role-gated
- [ ] Task: profile page "Mine køretøjer" section (list, edit, remove, add)

**Trailer half, in this order:**

- [ ] Task: shared-go — `types.VehicleKind` (`car`/`trailer`) with `Valid()`, a `kind`
      column `NOT NULL DEFAULT "car"`, `Kind` on `RegisterFields` and on
      `NathejkVehicleRegistered`, **and a projector default so a replay of pre-`kind`
      events yields `car`**
- [ ] Task: bump shared-go in `hej` and verify `GOWORK=off`
- [ ] Task: `hq` — filter dispatch/pickup views to `kind = car`, and bump shared-go
      there (separate repo; **prerequisite** for the next task, not a follow-up)
- [ ] Task: trailer registration in onboarding and on the profile page

## 11. Decisions

Answered questions are recorded here rather than deleted, so the reasoning survives.

- **2026-08-25 — Trailers are modelled by a `kind`/type property on shared-go's
  vehicle entity.** Full reasoning, the field's shape, and the replay-default trap are
  in §8. Two consequences worth repeating here because they land outside this repo:
  the change belongs in **shared-go** (nothing here forks the entity), and **`hq` must
  filter its dispatch views to `kind = car` before trailer registration is enabled**,
  or this feature regresses the coordinator's surface.

  Rejected: trailers as `seatCount = 0` vehicles, because that value already means "a
  car not offered for pickups" and the two facts would share one encoding.

  Left out deliberately: a `towedBy` relation. Knowing a trailer is present serves
  parking, insurance and site safety; knowing which car pulls it serves nothing anyone
  has asked for, and an unmaintained relation rots (§12.5).

## 12. Open Questions

1. **Who else may bring a vehicle?** The request names bandit, gøgler and crew. Are
   there adults in other roles — patrol leaders, guardians dropping off — whose
   vehicles are also "associated with the race" and therefore in scope for parking or
   insurance? They are not app users today, which may be the real answer.
2. **Is a plate the right required field?** It is the only reliable field-level
   identifier, but someone borrowing a car may not know it in advance. Allow a
   provisional registration without one, or hold the line?
3. **What does the inventory actually feed?** Dispatch is clear. Parking, access
   control and insurance were the stated motivations — do any of them need fields this
   PRD is not collecting (e.g. arrival date, expected departure, an insurance
   reference)? Better to know now than to re-ask 300 people later.
4. **Should trailers really be self-registered by anyone**, or are they mostly
   organiser-brought equipment that an organiser should enter?
5. **Will anyone ever need to know which car tows which trailer?** Left out per §11.
   Cheap to add later as a nullable reference; expensive to keep accurate if nothing
   depends on it.
6. **`kind` or `type`?** §8 chose `kind` to avoid the Go-keyword friction; `type` would
   match `types.TeamType`'s precedent. Trivially reversible before the shared-go change
   ships, awkward afterwards.
7. **Vehicle position.** Explicitly out of scope here, but `hq`'s transit flow knows a
   member is "in one of our cars" without knowing where that car is. If that is a real
   gap it deserves its own PRD, with the same care PRD 002 is applying to member
   position — not a field bolted onto this one.
