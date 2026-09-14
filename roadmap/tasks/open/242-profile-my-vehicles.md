# 242 — Profile: "Mine køretøjer"

**Status:** open
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

## Description

The section PRD 003 §17 reserved for this PRD, in `vue/src/views/ProfileView.vue`: the
caller's vehicles, with edit, remove, and an add action.

This is the surface that makes registration trustworthy. Someone who skipped at onboarding
registers here, and someone whose plans changed corrects here — **without a nag implying
they did something wrong** (PRD 010 §5). No badge, no warning icon, no "incomplete
profile". Not bringing a car is a perfectly ordinary state, and the app has no way of
knowing whether a missing registration means "did not bother" or "came by train".

- Each vehicle: plate, plus a summary line from brand/model/colour and the seat count.
- Remove asks for confirmation — a plate is quick to delete and slow to retype, and the
  consequence of an accidental deletion is a car the coordinator cannot dispatch.
- Role-gated identically to the onboarding step: every role except spejder. A spejder does
  not see the section at all, rather than seeing an empty one.
- shadcn-vue (`Card`, `Dialog` or `Drawer` for the edit form, `Button`), Lucide (`Car`,
  `Plus`, `Trash2`), `font-nathejk` on the section heading only, Danish copy.

Depends on task 240. Trailer display is task 246.

## Acceptance Criteria

- [ ] A "Mine køretøjer" section on the profile page, for every role except spejder
- [ ] Absent — not empty — for a spejder
- [ ] Lists each vehicle with plate and a summary line
- [ ] Add, edit and remove all work against the store
- [ ] Remove is confirmed before it fires
- [ ] No nag, badge or warning for a user with no vehicles
- [ ] An empty state that reads as ordinary, with the add action available
- [ ] shadcn-vue + Lucide only
- [ ] Component tests: list, empty state, spejder absence, remove confirmation
- [ ] `npm run type-check`, `npm run lint` and `npm run test:unit` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
