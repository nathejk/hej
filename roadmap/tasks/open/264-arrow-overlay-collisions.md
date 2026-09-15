# 264 — Arrow overlay collision rules against the existing map controls

**Status:** open
**Priority:** low
**Created:** 2026-09-15

## Description

PRD 016 phase 3. `MapsView.vue` already floats three things over the map: the control stack
(layer switcher, locate) top-right, notices top-left, and the registrations handle
bottom-centre — all in the `z-10` overlay layer, positioned against `--sat` / `--sab` safe
areas.

Edge arrows are pinned to the viewport edge, which is exactly where those controls live. An
arrow that lands under the locate button is worse than no arrow: it is invisible *and* it
steals the tap.

Define and implement the safe region: the **vertical middle band** of each edge, excluding
the top strip occupied by controls and notices and the bottom strip occupied by the handle
and the nav. Arrows that would fall outside it clamp into it rather than being dropped —
direction is approximate anyway at the edge, and losing an arrow loses information.

Worth checking against `LayoutDebug.vue`, which exists for exactly this class of problem,
and on a device with a notch.

## Acceptance Criteria

- [ ] Safe region defined in one place, derived from the same safe-area variables the
      controls use.
- [ ] No arrow overlaps the control stack, the notices, or the bottom handle (test).
- [ ] Arrows clamp into the band rather than disappearing.
- [ ] Verified on a notched device in both orientations.
- [ ] Arrows never intercept a tap meant for a control.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 3.
