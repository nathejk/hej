# 181 — Shared candidate-list component

**Status:** done
**Priority:** medium
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

From **PRD 012** §8. Extract the candidate list from `WelcomeStepLogin.vue` into a component both
the login chooser and the profile switcher render.

The list is currently inline in the onboarding step: a button per candidate showing the name, with
patrol or section beneath it. That secondary line is the part that actually disambiguates — two
siblings share a patrulje but not a name, two crew may share a name but not a section — and it is
exactly the detail that would drift if the switcher grew its own copy.

Two surfaces that could disagree about how a person is identified is one too many, especially when
one of them is how somebody proves which of two profiles is theirs.

## Implementation

`vue/src/components/auth/ProfileChooser.vue`, used by `WelcomeStepLogin.vue` and `UserMenu.vue`.

Takes candidates, emits a user id, and knows nothing about why it is being shown. The login flow keeps
its own "Skift nummer" affordance, which stays where changing number is a meaningful action.

One prop the extraction needed: `dark`. The login chooser sits on the light onboarding card, the
switcher's dialog on a dark surface. A boolean beats two copies of the markup, and beats hardcoding
colours that would be wrong on one of the two.

`ChevronRight` moved with it, and dropped from `WelcomeStepLogin`'s imports — worth mentioning because
an unused import there would have been the only trace of the old inline list.

## Acceptance Criteria

- [x] A component taking candidates and emitting the chosen user id, with no knowledge of *why* it is
      being shown.
- [x] `WelcomeStepLogin.vue` uses it, with no visual change to the login flow — same markup, same
      classes, same disabled behaviour.
- [x] The switcher's dialog uses the same component.
- [x] A disabled/busy state, since both callers await a request on selection.
- [x] Nothing about onboarding leaks into it.

## Progress Log

- 2026-09-01 — Task created from PRD 012 §10.
- 2026-09-01 16:10 — Extracted. Added a `dark` prop rather than duplicating the markup for the two
  surfaces. ✅ All criteria met; `vue-tsc` clean and the login flow's markup is unchanged.
