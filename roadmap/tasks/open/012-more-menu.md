# 012 — `MoreMenu` overflow sheet

**Status:** open
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Build the overflow menu opened by the 5th "More" (burger) slot, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. A bottom sheet / panel
lists the remaining allowed destinations (icon + label); selecting one navigates
and closes the sheet.

Depends on: 011.

## Acceptance Criteria

- [ ] `src/components/MoreMenu.vue` lists destinations beyond the 4th (allowed,
      role-filtered).
- [ ] Selecting an item navigates and closes the sheet; active item indicated.
- [ ] Keyboard + screen-reader reachable; respects safe-area insets.
- [ ] Open/closed state lives in `app.store` (per PRD).

## Progress Log

- 2026-07-30 13:12 — Task created.
