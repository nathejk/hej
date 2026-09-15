# 264 — Arrow overlay collision rules against the existing map controls

**Status:** done
**Priority:** low
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

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

- [x] Safe region defined in one place, derived from the same safe-area variables the
      controls use.
- [x] No arrow overlaps the control stack, the notices, or the bottom handle (test).
- [x] Arrows clamp into the band rather than disappearing.
- [ ] Verified on a notched device in both orientations — **needs a device.** Composed from the
      real `--sat`/`--sab` values and tested against a notched phone's readings (59/34), but
      "looks right on an iPhone in landscape" cannot be claimed from a test run. Together with
      task 263's stutter check, this is the outstanding device pass for phase 3.
- [x] Arrows never intercept a tap meant for a control.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 3.
- 2026-09-15 — Part of this landed with task 263, since the clamping is inseparable from the placement: an
  arrow is positioned once, and "where it may not go" is an input to that, not a later correction.
- 2026-09-15 — **Corrected my own first attempt.** I had written the keep-out as a single hardcoded constant
  with a comment justifying it as "deliberately not measured". That justification did not survive this task's
  first acceptance criterion, and the criterion was right: the controls are positioned with
  `calc(var(--sat) + 0.75rem)`, so their real extent depends on the device. `--sat` is 59 px on the
  maintainer's notched iPhone and 0 in a desktop browser — one number would either waste a third of the screen
  or put arrows under the layer switcher on every real phone.

  Now composed: `CONTROL_EXTENT` (what the controls themselves occupy) plus the live `--sat`/`--sab`, read from
  the same custom properties the controls use. The composition is a pure function (`arrowKeepOut`) so it is
  testable without a browser — the same split `safeArea.insetVars` makes for the same reason.
- 2026-09-15 — **A test found a real bug in that function.** `Math.max(0, NaN)` is `NaN`, so a non-finite
  reading would have propagated into a CSS `top` and placed the arrow nowhere. Not hypothetical:
  `parseFloat('')` on an unset custom property is exactly `NaN`. Replaced the clamp with an explicit
  finite-and-non-negative check.
- 2026-09-15 — Added a band-width assertion for the shortest viewport we support (~320 px in landscape): the
  band has to leave room for the arrow's own radius on both sides, or the clamp has nothing to clamp into. It
  is tight, which is why it is asserted rather than assumed.
- 2026-09-15 — Tap-stealing is covered **structurally**, following `layout.spec.ts` and
  `offlineIndicator.spec.ts`: the overlay must be `pointer-events-none` with `pointer-events-auto` only on the
  arrows, each arrow must be a real `<button>` with an `aria-label`, and the chevron must be `aria-hidden`. A
  tappable `div` would look identical and be unreachable without a pointer; an overlay that swallowed events
  would be reported as "the map froze".
- 2026-09-15 — ✅ 8 further specs. Frontend: 597 tests, type-check and build clean. Go suite clean.
- 2026-09-15 — Done. **Phase 3 is complete apart from one honest gap**: the two device-only checks (no stutter
  on pan/zoom, notched device in both orientations). Everything decidable without hardware is decided and
  tested.
