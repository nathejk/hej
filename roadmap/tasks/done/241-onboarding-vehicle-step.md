# 241 — Onboarding: the vehicle step

**Status:** done
**Priority:** high
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

## Description

Fill the `vehicle` slot that `vue/src/stores/onboarding.store.ts` has been holding open
for this PRD. The store's comment describes the position and the shape, and the machine was
built declaratively precisely so this lands as a data change rather than new control flow —
add a `StepDescriptor`, do not add branching.

- **Position:** after `portrait`, before `location`. It is another "about you" question,
  not a device prompt.
- **Applies to:** every role except spejder — `allRolesExcept('spejder')`, or equivalently
  `role !== 'spejder'`. Write it as an exclusion, not an allow-list: `config/roles.ts`
  already carries that lesson in the comment on `allRolesExcept`, and an allow-list would
  silently deny a role added later.
- **Settled when:** the user has answered the question — either registered a vehicle or
  said they are not bringing one. Note that "settled" here cannot be derived from the
  platform the way `location` and `notifications` can, so a "no" answer is a session-level
  skip (`skip('vehicle')`), consistent with how the store already treats "not now" and
  explicitly **not** persisted as "never" (PRD 005 §11).
- **Mandatory:** no. Login is the only mandatory step and that must not change — nothing
  about a car may keep a participant out of a safety app.

UI, per PRD 010 §7:

- A yes/no gate first, so the majority who bring nothing answer in one tap and never see a
  form.
- Then the form, ordered by what people know without looking: plate, brand/model/colour,
  seats.
- **Seat count must make "excluding the driver" unmissable** — not a hint below the field.
  An off-by-one here means a coordinator dispatches a car with one seat too few at the
  worst possible moment.
- shadcn-vue components only (prefer an existing primitive over hand-rolling), Lucide icons
  (`Car`, `Plus`), `font-nathejk` for the step heading only, Danish copy.

New component under `vue/src/components/onboarding/`, following the existing
`WelcomeStep*.vue` files.

Depends on task 240. Trailer support is task 246 — build the step so adding it does not
mean rewriting it, but do not add it here.

## Acceptance Criteria

- [x] A `vehicle` entry added to `STEPS` as data; no new branching in the step machine
- [x] Sits after `portrait` and before `location`
- [x] Applies to every role except spejder, expressed as an exclusion
- [x] Absent for a spejder — asserted in `onboarding.store.spec.ts`
- [x] Not mandatory; declining never blocks the flow
- [x] "No" is one tap and does not show the form
- [x] A declined answer does not re-present itself for the rest of the flow, and is not
      persisted as a permanent "never"
- [x] Seat count's "excluding the driver" is part of the label, not a hint
- [x] shadcn-vue + Lucide only; no hand-rolled equivalents of existing primitives
- [x] A failed submit keeps the user on the step with their input intact
- [x] Store tests cover the new step's applicability and settled predicates
- [x] `npm run type-check` and the unit suite green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up. Plan: a `StepDescriptor` entry plus a component, no new branching;
  the vehicles store has to be pre-loaded like the profile is, or `applies` decides against an
  empty store and re-asks somebody who already registered.
- 2026-09-14 — The store change is exactly what the slot comment promised: **one
  `StepDescriptor` entry**, plus two new context fields. No branching added anywhere. Worth
  noting because it is the payoff of task 118's decision to write the sequence as data — the
  machine that has now absorbed a whole PRD's step did not have to be reasoned about.
- 2026-09-14 — `applies` is `mayRegisterVehicle && !hasVehicle`. The second half is what stops
  the app asking a member about a car they registered last week, and it is also what makes the
  step depend on a loaded store — so `WelcomeView.loadProfile` now pre-loads vehicles too,
  next to the profile it already loads for exactly this reason, and skips the request for
  spejdere (the same role gate the contacts prefetch uses, so a few hundred phones do not ask
  for something the server will always refuse).
- 2026-09-14 — `settled` is "has a vehicle on file", and answering **no** is a session skip
  rather than settlement. There is nothing to store for "I am not bringing a car" and nothing
  to read back, so persisting it would turn "not now" into "never" — the mistake PRD 005 §11
  already rejected for the portrait, and the member who ends up borrowing a car on the day is
  exactly who it would fail.
- 2026-09-14 — Seat count is bound to a **string**, not a number. An `<input type="number">`
  bound to a number reads an empty field as 0, and 0 is a meaningful answer here ("brought only
  for my own transport, not available for pickups") — it has to be something the member chose,
  not something an empty field produced.
- 2026-09-14 — A `409` gets its own affordance: "the car is already registered — continue".
  For the likeliest cause (a passenger filling in the form after the driver already did) that
  is a success, and offering only a retry would leave them stuck on a step that cannot succeed.
- 2026-09-14 — Used `Button` variants for the secondary actions rather than the bare
  `<button class="text-sm text-slate-500">` the neighbouring steps use. `.rules` says to prefer
  a shadcn primitive wherever one exists; the older steps predate that and are not mine to
  churn, but new markup should not add to them.
- 2026-09-14 — The existing "does not contain the PRD 009 / PRD 010 slots yet" test failed, as
  designed — that assertion existed to catch exactly this landing. Replaced with a full-sequence
  test (`login → portrait → vehicle → location → notifications`) plus a narrower one still
  guarding PRD 009's slot.
- 2026-09-14 — ✅ All criteria. Seven new store tests, including the spejder absence and one
  asserting `crew` and `samarit` *do* get the step — two roles no "bandit/gøgler/crew"
  allow-list would have named, which is the exclusion rule earning its keep. Full suite green
  (510 tests), `type-check` clean.
