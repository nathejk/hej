# 033 — Record the shadcn-vue / no-PrimeVue rules in .rules

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Write the component-library decision into `.rules` so agents and developers are
told about it before they write UI, per PRD 004.

Completed ahead of PRD approval at the user's explicit request ("this should also
be stated in the repo rule file that this repo should use shadcn from now on, no
more primevue").

## Acceptance Criteria

- [x] `.rules` states shadcn-vue is the component library for all UI.
- [x] `.rules` states that a **standard** shadcn-vue component must be preferred
      whenever one exists, and that hand-rolling requires a comment saying why.
- [x] `.rules` states PrimeVue must not be used or reintroduced — no `primevue`,
      no `@primevue/*`, no Lara/theme presets, no PrimeIcons.
- [x] `.rules` states Tailwind v4, configured CSS-first, and that
      `tailwind.config.js` must not come back.
- [x] `.rules` points at the `vue3-pwa-layout` skill for `vue/` work.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004 (retroactively — work was done
  before the PRD was approved, at the user's request).
- 2026-08-24 00:00 — Added a **Component library** bullet (shadcn-vue,
  standard-component-first, `src/components/ui/` as owned source, explicit
  PrimeVue/PrimeIcons ban referencing PRD 004), a **Styling** bullet (Tailwind v4
  CSS-first, no `tailwind.config.js`), and a **Frontend skill** section pointing to
  `vue3-pwa-layout` and warning off `vue3-spa-layout-legacy`.
- 2026-08-24 00:00 — Flagged the Tailwind bullet as *target state per PRD 004,
  tree is still on 3.4* so the rule does not misrepresent the current code while
  tasks 024–030 are outstanding.
- 2026-08-24 00:00 — Completed. Note the docs now lead the code; that gap closes
  when 024–031 land.
