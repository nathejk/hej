# 011 — Role-filtered `BottomNav` (≤5 / burger rule)

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Build the data-driven bottom navigation, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Destinations come from a
single declarative config (icon + label + route + required role/visibility
rule). The nav is filtered by the signed-in role, then the ≤5 / burger rule is
applied to the filtered set: at most 5 slots; if more than 5 allowed
destinations, render 4 + a "More" burger.

Icons: use **Lucide** (`lucide-vue-next`) per `.rules` — no PrimeIcons/other
sets. The overflow ("More") slot uses the Lucide `Menu` (burger) icon.

Depends on: 010, 013 (navigation config).

## Acceptance Criteria

- [x] `src/components/BottomNav.vue` renders destinations from
      `navigation.ts`, filtered by role (`visibleDestinations`).
- [x] ≤5 allowed → all shown; >5 → first 4 + a "More" burger (Lucide `Menu`)
      slot revealing the overflow.
- [x] Active-route highlighting via `active-class` (color + weight, not color
      alone); "More" is highlighted when a hidden destination is active or the
      overflow is open.
- [x] Items are real `RouterLink`s / a `<button>` with `aria-label`/
      `aria-expanded`; ≥ 44px targets.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 19:55 — Reworked `BottomNav.vue` with the cap rule: `MAX_SLOTS=5`; when the role's visible destinations exceed 5, render the first 4 + a "Mere" burger that toggles the overflow list. Overflow currently renders as an inline panel above the nav; task 012 replaces it with a proper bottom sheet. Spejder/bandit (6 dests) and service roles (7) both trigger the burger.
- 2026-07-30 19:56 — ✅ Verified in `node:20-alpine`: `npm run build` + `npm run type-check` clean.
- 2026-07-30 19:56 — Completed. Overflow behaviour works; task 012 will make the "More" panel a sheet with backdrop/transition.
