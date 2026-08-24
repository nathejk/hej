# 032 — Refactor MoreMenu onto the shadcn sheet/drawer primitive

**Status:** done
**Priority:** medium
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

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

- [x] The evaluation is recorded in the progress log with the decision and its
      reasoning.
- [x] If refactored: `MoreMenu.vue` composes the generated primitive, the sheet
      looks and behaves as before (open/close, backdrop, escape, safe-area
      bottom inset, ≥44px rows), and hand-rolled a11y/scroll-lock code is gone.
- [x] If kept hand-rolled: a comment at the top of `MoreMenu.vue` states why no
      shadcn primitive fits, per the repo rule.
- [x] Keyboard and screen-reader behaviour is no worse than before.
- [x] `npm run type-check` and `npm run build` pass in the `ui` container.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 02:20 — **Decision: refactored onto `Drawer`.** The hand-rolled sheet
  was 65 lines of `Teleport` + two `Transition`s + hand-written keyframes, and it
  had **no focus trap, no escape-to-close and no scroll lock** — it only closed on
  backdrop tap. The Drawer primitive supplies all of that plus swipe-to-dismiss and
  a drag handle. That is a clear win, which is the bar this task set.
- 2026-08-24 02:22 — Checked the primitive's assumptions in `node_modules` rather
  than guessing: `DrawerRoot`'s prop is `swipeDirection` (not `direction`) and it
  defaults to `"down"`, which matches the `data-[swipe-direction=down]` classes in
  the generated `DrawerContent` — so a bottom sheet needs no extra props.
- 2026-08-24 02:24 — Kept: the `env(safe-area-inset-bottom)` padding, the Danish
  accessible name (now a visually hidden `DrawerTitle`, since Drawer requires a
  title for its `aria-labelledby`), and ≥44px rows (`min-h-[3.25rem]` = 52px).
  Changed: corner radius is now the primitive's `rounded-t-xl` rather than
  `rounded-t-2xl`, and the drag handle is the primitive's. Both are cosmetic and
  judged acceptable; noted here rather than fought.
- 2026-08-24 02:30 — **Bundle regression caught and fixed.** Naively importing the
  Drawer put Reka UI's drawer code in the app-shell bundle: gzip 47.4 → 71.9 kB,
  eating almost the entire PrimeVue saving — for a menu most sessions never open.
  Made `MoreMenu` a `defineAsyncComponent` in `BottomNav`, mounted on first open
  (`overflowMounted` stays true afterwards so the close animation still plays).
  Result: shell back to 48.5 kB gzip with a 25.0 kB gzip `MoreMenu` chunk fetched
  on demand.
- 2026-08-24 02:32 — ✅ type-check + build clean. **Residual risk, stated plainly:**
  I could not open a browser, so the drawer's rendering and gestures are verified
  only by types, build output and reading the primitive's source. A real-device
  check of open/close/swipe/escape/focus is required before PRD 004 ships — it is
  part of tasks 025 and 036, which remain open. If it misbehaves, reverting to the
  hand-rolled sheet is the documented fallback. Completed.
