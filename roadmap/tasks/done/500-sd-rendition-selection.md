# 500 — SD rendition selection and HD toggle

**Status:** done
**Priority:** medium
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

For clips with a 480p rendition: start on SD when `effectiveType` is 3g or lower, or switch after a stall > 2 s, keeping position. A manual HD/SD toggle is always available.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Heuristic verified on a throttled network — in headless Chrome with CDP network emulation and a short sample clip; a real 22-min clip on a real phone is task 504
- [x] Switch keeps playback position

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — `viewer.js`: a clip with `data-sd` starts on 480p when `navigator.connection` reports save-data or an effectiveType of 3g or slower (Chromium only; elsewhere the stall rule is the protection), otherwise on 720p. A `waiting` event starts a 2 s timer; if the buffer is still below HAVE_FUTURE_DATA, it switches to 480p for the rest of the viewing. An HD/SD button (≥ 44 px, aria-pressed, Danish labels) appears only for clips with an SD rendition; a manual choice holds for the viewing in either direction. Switching keeps position and play state.
- 2026-10-04 — CDP checks: no toggle without SD; starts HD on a fast link; toggle → SD at the same position, still playing; emulated 3g → starts SD; no switch before 2 s; switch after a simulated 2 s stall. The task 499 suite still passes 19/19.
