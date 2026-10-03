# 491 — ffmpeg and ffprobe in the API image

**Status:** open
**Priority:** high
**Created:** 2026-10-03
**Picked up by:**
**Started:**
**Completed:**

## Description

Add ffmpeg/ffprobe to the API Docker image. Measure the image size growth and the time to transcode a 22-min 2026 clip on the production host with the PRD 029 §8.1 settings.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [ ] ffmpeg and ffprobe available in the API image
- [ ] Image size delta recorded
- [ ] 22-min transcode time on the prod host recorded (target ≤ 2× real time)

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
