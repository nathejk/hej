# 493 — Video events on the PHOTO stream and their folds

**Status:** open
**Priority:** high
**Created:** 2026-10-03
**Picked up by:**
**Started:**
**Completed:**

## Description

`video.uploaded`, `video.transcoded` and `video.failed` on the PHOTO stream, folded into the `photo` table. Refs are validated as photo refs are.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [ ] Events defined and published on the PHOTO stream
- [ ] Folds write status/refs/duration; invalid refs blanked
- [ ] Re-upload of the same original lands on the same row

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
