# 104 — BFF: server-side thumbnail generation at upload

**Status:** open
**Priority:** low
**Created:** 2026-08-28

## Description

Depends on task 103. (Task 102's consent blocker was cleared 2026-08-28.)

PRD 007 syncs portrait thumbnails to devices for offline identification, so the
thumbnail must be produced once at upload rather than per request: EXIF-correct
orientation, a fixed size, stored alongside the full image.

## Acceptance Criteria

- [ ] A fixed-size thumbnail is generated and stored on every portrait write.
- [ ] Orientation is correct for photos taken in every device orientation.
- [ ] Generation failure fails the upload rather than storing a portrait with no
      thumbnail.
- [ ] Test with a rotated fixture image.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
