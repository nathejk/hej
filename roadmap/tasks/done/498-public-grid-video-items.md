# 498 — Public album grid: poster, play glyph and duration

**Status:** done
**Priority:** medium
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

Album grid shows the poster, a play glyph and the duration (m:ss / h:mm:ss) for video items. No video bytes are loaded in the grid.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Poster, glyph and duration shown
- [x] Zero video requests from the grid

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — `publicAlbumItem` gains IsVideo/HasSd/DurationMs. The tile keeps its poster `<img>` (the thumb variant is the poster) and adds a `▶ m:ss` badge with an aria-label, plus `data-kind`, `data-duration-ms` and `data-sd` for the viewer (task 499). No `<video>` in the grid, asserted by `TestAlbumGridShowsAVideoAsItsPoster`; the tile's href is the 720p MP4, so without script the link plays the clip in the browser.
