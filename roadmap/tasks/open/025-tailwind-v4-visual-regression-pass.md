# 025 — Visual regression pass after the Tailwind v4 upgrade

**Status:** open
**Priority:** high
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

## Description

Tailwind v4 changes several defaults, and this repo has **no unit or e2e test
suite** — `vue/package.json` exposes only `dev`, `build`, `preview` and
`type-check`. Verification is therefore manual and this task exists to make that
explicit rather than implicit.

After task 024, walk every route on a phone viewport (and, ideally, the installed
PWA on a real device) and fix drift introduced by v4's changed defaults:

- default border colour/width and ring colour/width,
- the shadow scale (`shadow-sm`/`shadow` shifted),
- `outline-none` → `outline-hidden`,
- `space-x/y` selector change,
- any removed/renamed utility the codemod missed.

Routes: `/login`, `/maps`, `/contacts`, `/rulebook`, `/updates`, `/schedule`,
`/faq`, `/sos`. Also exercise the `MoreMenu` overflow sheet, the location and
notification pre-prompts (`PermissionPrompt.vue`), and `UpdatePrompt.vue`.

Take before/after screenshots where a difference is judged acceptable, and log the
decision — "looks slightly different and that's fine" must be a recorded choice,
not an accident.

PRD: 004. Depends on: 024.

## Acceptance Criteria

- [ ] Every route listed above renders with no console errors and no unintended
      visual change on a phone viewport.
- [ ] The `MoreMenu` bottom sheet, both permission pre-prompts and the update
      prompt are verified specifically (borders, shadows, rings, focus rings).
- [ ] Safe-area insets still behave on a notched viewport (`App.vue` header,
      `BottomNav.vue`).
- [ ] Any accepted visual difference is logged with a one-line rationale.
- [ ] Focus-visible styling is still present on all interactive elements.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
