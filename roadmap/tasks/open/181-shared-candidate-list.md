# 181 — Shared candidate-list component

**Status:** open
**Priority:** medium
**Created:** 2026-09-01

## Description

From **PRD 012** §8. Extract the candidate list from `WelcomeStepLogin.vue` into a component both
the login chooser and the profile switcher render.

The list is currently inline in the onboarding step: a button per candidate showing the name, with
patrol or section beneath it. That secondary line is the part that actually disambiguates — two
siblings share a patrulje but not a name, two crew may share a name but not a section — and it is
exactly the detail that would drift if the switcher grew its own copy.

Two surfaces that could disagree about how a person is identified is one too many, especially when
one of them is how somebody proves which of two profiles is theirs.

## Acceptance Criteria

- [ ] A component taking candidates and emitting the chosen user id, with no knowledge of *why* it
      is being shown.
- [ ] `WelcomeStepLogin.vue` uses it, with no visual change to the login flow.
- [ ] The switcher's dialog uses the same component.
- [ ] A disabled/busy state, since both callers await a request on selection.
- [ ] Nothing about onboarding leaks into it — no step logic, no "Skift nummer" (that stays in the
      login flow, where changing number is meaningful).

## Progress Log

- 2026-09-01 — Task created from PRD 012 §10.
