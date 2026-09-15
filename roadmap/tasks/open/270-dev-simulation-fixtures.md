# 270 — Dev-simulation fixtures for every verdict and reveal state

**Status:** open
**Priority:** low
**Created:** 2026-09-15

## Description

PRD 016 phase 4, using PRD 014's device-simulation machinery. Most of this feature's states
are unreachable in development without a race happening: a relative window with an anchoring
scan, a sheet reassigned to a successor team, a skitse revealed at a post, a scan nobody can
attribute.

Seed the `internal/scans` mock and the dev fixtures so every state can be looked at on a
device:

**Verdicts:** on time; late with a delta; early; `fixed` window; `relative` window with an
anchor; `relative` without an anchor (no verdict); `none` (no verdict); unattributable scan
(no checkpoint at all).

**Reveal states:** revealed by QR-bound sheet; revealed by a sheet's handout checkgroup;
revealed by having scanned a checkgroup; a positioned checkpoint revealed by nothing (must
never appear); a revealed checkpoint with no position (listed nowhere on the map); a sheet
with an unknown `mapId`; a reassigned sheet whose checkpoints stay revealed.

**Handouts:** QR-bound; synthesised skitse; "afleveret".

This is also the fixture set the reveal regression test (task 258) and the arrow tests
(task 263) want, so build it where both can reach it.

## Acceptance Criteria

- [ ] Every verdict state reachable in dev.
- [ ] Every reveal state reachable in dev, including the must-never-appear one.
- [ ] Fixtures shared with tasks 258 and 263 rather than duplicated.
- [ ] Documented in the dev simulation notes so the states are discoverable.
- [ ] Fixtures cannot leak into production builds.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 4.
