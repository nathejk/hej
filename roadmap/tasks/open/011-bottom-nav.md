# 011 — Role-filtered `BottomNav` (≤5 / burger rule)

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Build the data-driven bottom navigation, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Destinations come from a
single declarative config (icon + label + route + required role/visibility
rule). The nav is filtered by the signed-in role, then the ≤5 / burger rule is
applied to the filtered set: at most 5 slots; if more than 5 allowed
destinations, render 4 + a "More" burger.

Depends on: 010, 013 (navigation config).

## Acceptance Criteria

- [ ] `src/components/BottomNav.vue` renders destinations from
      `src/config/navigation.ts`, filtered by role.
- [ ] ≤5 allowed destinations → all shown; >5 → 4 items + "More" burger slot.
- [ ] Active-route highlighting (not by color alone; accessible name present).
- [ ] Items are real links/buttons; keyboard + screen-reader reachable.

## Progress Log

- 2026-07-30 13:12 — Task created.
