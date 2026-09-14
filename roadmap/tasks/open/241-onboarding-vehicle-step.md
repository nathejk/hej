# 241 — Onboarding: the vehicle step

**Status:** open
**Priority:** high
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A `vehicle` entry added to `STEPS` as data; no new branching in the step machine
- [ ] Sits after `portrait` and before `location`
- [ ] Applies to every role except spejder, expressed as an exclusion
- [ ] Absent for a spejder — asserted in `onboarding.store.spec.ts`
- [ ] Not mandatory; declining never blocks the flow
- [ ] "No" is one tap and does not show the form
- [ ] A declined answer does not re-present itself for the rest of the flow, and is not
      persisted as a permanent "never"
- [ ] Seat count's "excluding the driver" is part of the label, not a hint
- [ ] shadcn-vue + Lucide only; no hand-rolled equivalents of existing primitives
- [ ] A failed submit keeps the user on the step with their input intact
- [ ] Store tests cover the new step's applicability and settled predicates
- [ ] `npm run type-check`, `npm run lint` and `npm run test:unit` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
