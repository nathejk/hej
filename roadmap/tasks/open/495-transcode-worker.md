# 495 — Transcode worker

**Status:** open
**Priority:** high
**Created:** 2026-10-03
**Picked up by:**
**Started:**
**Completed:**

## Description

A dedicated goroutine in the API binary, one job at a time, `nice`d. It produces the 720p MP4, the 480p MP4 for clips over 5 min, and posters at the thumb and medium sizes (cache class). Jobs are idempotent per original ref and survive restart; a rendition cache miss re-queues the job. Emits `video.transcoded`/`video.failed`.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [ ] Renditions match the PRD 029 §6 settings (faststart, metadata stripped, rotation applied)
- [ ] Restart mid-job resumes
- [ ] Cache miss rebuilds
- [ ] Retry endpoint for failed items, with OpenAPI annotations

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
