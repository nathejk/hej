# 032 — Refactor MoreMenu onto the shadcn sheet/drawer primitive

**Status:** open
**Priority:** medium
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

## Description

`vue/src/components/MoreMenu.vue` is a hand-rolled bottom sheet holding the
navigation destinations beyond the 4th (see PRD 001, task 012). Now that a
generated `sheet`/`drawer` primitive exists (task 029), evaluate replacing the
hand-rolled implementation with it.

This is the repo's first application of the standard-component-first rule now in
`.rules` and the `vue3-pwa-layout` skill, so it doubles as a proof that the rule
is workable.

Refactor **only if** it is a clear win — i.e. it removes hand-rolled focus
trapping, ARIA wiring, scroll-locking or escape/backdrop handling — and **only
if** the appearance and interaction stay the same. If the primitive fights the
mobile bottom-sheet layout or the safe-area insets more than it helps, keep the
hand-rolled version and instead add the comment the rule requires explaining why
it is hand-rolled.

Either outcome is a valid completion of this task; the point is that the choice
becomes deliberate and documented.

PRD: 004. Depends on: 029.

## Acceptance Criteria

- [ ] The evaluation is recorded in the progress log with the decision and its
      reasoning.
- [ ] If refactored: `MoreMenu.vue` composes the generated primitive, the sheet
      looks and behaves as before (open/close, backdrop, escape, safe-area
      bottom inset, ≥44px rows), and hand-rolled a11y/scroll-lock code is gone.
- [ ] If kept hand-rolled: a comment at the top of `MoreMenu.vue` states why no
      shadcn primitive fits, per the repo rule.
- [ ] Keyboard and screen-reader behaviour is no worse than before.
- [ ] `npm run type-check` and `npm run build` pass in the `ui` container.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
