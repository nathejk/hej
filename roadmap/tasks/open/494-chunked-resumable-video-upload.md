# 494 — Chunked, resumable admin video upload

**Status:** open
**Priority:** high
**Created:** 2026-10-03
**Picked up by:**
**Started:**
**Completed:**

## Description

Admin upload sessions: create, PUT chunks with Content-Range, GET offset, complete (PRD 029 §8.4). Caps: ≤ 30 min (ffprobe) and ≤ 4 GB. The original is stored byte for byte in the backed-up class. Incomplete uploads are cleaned up after 24 h.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [ ] Upload resumes after interruption
- [ ] 413 over 4 GB; refusal over 30 min names the limit
- [ ] Original stored and `video.uploaded` emitted
- [ ] OpenAPI annotations for every new endpoint

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
