# 242 — Profile: "Mine køretøjer"

**Status:** done
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

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

- [x] A "Mine køretøjer" section on the profile page, for every role except spejder
- [x] Absent — not empty — for a spejder
- [x] Lists each vehicle with plate and a summary line
- [x] Add, edit and remove all work against the store
- [x] Remove is confirmed before it fires
- [x] No nag, badge or warning for a user with no vehicles
- [x] An empty state that reads as ordinary, with the add action available
- [x] shadcn-vue + Lucide only
- [x] Tests: labels as functions, plus structural assertions for the rules that live in the
      template — see the log on why not mounted
- [x] `npm run type-check` and the unit suite green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up. `MyVehicles.vue` under `components/profile/`, dropped into
  `ProfileView` between the read-only registration details and the device settings — it is the
  one thing on that page a member can actually add about themselves.
- 2026-09-14 — One dialog serves both add and edit, since the fields are identical and two forms
  would drift. On a failed save the dialog stays open with the fields as typed: the member has
  just read a plate off a car, and closing it to show an error would throw that away.
- 2026-09-14 — Removal is confirmed in a `Dialog` rather than `confirm()`. The native one is
  suppressible, unstyled, and on iOS names the site rather than the car — and the consequence of
  an accidental deletion is a vehicle the coordinator can no longer send out.
- 2026-09-14 — Dropped `size="sm"` from the row actions. The button index says outright that
  `xs`/`sm` are opt-in for dense secondary affordances and that the local variants were bumped to
  a ≥44px target for exactly this kind of surface (task 010); a 32px "Ret" in a list on a phone
  used one-handed outdoors would have quietly undone that.
- 2026-09-14 — **Amended the test criterion, which I had written wrong.** This project has no
  jsdom and no `@vue/test-utils` (`vitest.config.ts` keeps `environment: 'node'` deliberately),
  so "component tests" as described were not possible. Followed the existing honest pattern from
  `contactCheck.spec.ts` and `layout.spec.ts`: extracted the label logic into
  `helpers/vehicleLabels.ts` and tested it as functions, then asserted the template-only rules
  against the source — the role gate being on the `<section>` itself, the absence of nag wording,
  the confirmation dialog, and the failed-read-vs-empty-list distinction.
- 2026-09-14 — `seatsLabel` is worth having as a tested function rather than inline string
  interpolation: every phrasing must carry "ud over dig selv", because `seatCount` excludes the
  driver and a label read as total capacity sends a car with one seat too few. Pinned by a test
  that asserts the qualifier is present for every plural form.
- 2026-09-14 — ✅ All criteria. 9 new tests; full suite green (519), `type-check` clean.
