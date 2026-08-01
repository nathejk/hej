# 012 — `MoreMenu` overflow sheet

**Status:** done
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Build the overflow menu opened by the 5th "More" (burger) slot, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. A bottom sheet / panel
lists the remaining allowed destinations (icon + label); selecting one navigates
and closes the sheet.

Depends on: 011.

## Acceptance Criteria

- [x] `src/components/MoreMenu.vue` lists the overflow destinations (beyond the
      4 primary, role-filtered) passed in by `BottomNav`.
- [x] Selecting an item navigates and closes the sheet; active item indicated;
      backdrop tap closes it.
- [x] `Teleport`ed to body with backdrop + slide-up/fade transitions;
      `role="dialog"`; respects `env(safe-area-inset-bottom)`.
- [x] Open/closed state managed by `BottomNav` (local ref) and passed via
      `open` prop + `close` emit. *(Kept in the component rather than app.store;
      no other consumer needs it yet.)*

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 20:05 — Implemented `MoreMenu.vue` as a controlled bottom sheet (Teleport + backdrop + slide-up/fade transitions, drag-handle affordance, `role="dialog"`). Replaced BottomNav's inline overflow panel with `<MoreMenu :open :items @close>`.
- 2026-07-30 20:05 — Decision: kept overflow open/closed as local state in BottomNav (props/emit to MoreMenu) instead of app.store — no other consumer needs it; can hoist later if required. Transition classes use a small `<style scoped>` (Tailwind can't express Vue transition states).
- 2026-07-30 20:06 — ✅ Verified in `node:20-alpine`: `npm run build` + `npm run type-check` clean.
- 2026-07-30 20:06 — Completed. App shell + role-filtered nav + burger overflow sheet are done; the shell is feature-complete for the skeleton.
