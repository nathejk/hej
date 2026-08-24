# 041 — Layer switcher with persisted choice

**Status:** done
**Priority:** medium
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Touch-friendly control for picking the base layer, persisting the choice across
navigation and reloads (PRD 002).

## Acceptance Criteria

- [x] Compact button (Lucide `Layers`) opening a panel with the three layers as
      radio options, showing which is active.
- [x] ≥44px targets; safe-area aware placement.
- [x] Choice persisted in `localStorage` under `hej.map.*`.
- [x] Switching preserves centre, zoom, position marker and scan markers.

## Progress Log

- 2026-08-24 11:35 — Built `LayerSwitcher.vue` with `defineModel`, rows ≥48px, and
  the 1:50.000 currency caveat rendered as sub-text under the label.
- 2026-08-24 11:36 — **Deliberate hand-roll, per the `.rules` escape hatch.** A
  shadcn `Popover`+`RadioGroup` would portal the panel to the body and centre it,
  which fights a control anchored inside the map's overlay stack; and Leaflet's own
  layers control is desktop-sized. Reason recorded in a comment at the top of the
  component, as the rule requires. Standard components are still used for the
  registrations sheet (task 046).
- 2026-08-24 13:05 — ✅ Verified in-browser: the button hit-tests as the topmost
  element at 44×44, switching to `dtk50` then `orto` fetched 8 tile responses each
  from `dtk_50_DAF` and `orto_foraar_DAF` respectively with zero non-image
  responses, and `hej.map.baseLayer` still read `orto` after a full reload.
