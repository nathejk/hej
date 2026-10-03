# 499 — Viewer: video playback, sound rules, loop and keyboard

**Status:** open
**Priority:** high
**Created:** 2026-10-03
**Picked up by:**
**Started:**
**Completed:**

## Description

In the PRD 023 viewer: autoplay with sound when opened by a click or tap (fall back to muted if rejected); muted when arrived at by swipe, with an unmute button; unmute persists for the session. Clips ≤ 15 s loop. Space toggles pause, ←/→ seek ±10 s, M toggles mute (ignored in form fields). Only the visible item plays; leaving pauses and releases the source. Swipes on the scrubber seek rather than change item.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [ ] Sound and mute rules as described, verified on iOS Safari and Android Chrome
- [ ] Loop ≤ 15 s
- [ ] Keyboard shortcuts work and Space does not scroll
- [ ] Leaving an item stops it

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
