# 496 — Seekable blob read and Range on media routes

**Status:** open
**Priority:** high
**Created:** 2026-10-03
**Picked up by:**
**Started:**
**Completed:**

## Description

`blob.Store` gains a seekable read; media routes use `http.ServeContent` (206, Accept-Ranges, 416). Shared with PRD 020 §8.4. `?q=480` selects the SD rendition. Must not regress ETag/immutable caching.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [ ] Range works on FileStore and MemoryStore
- [ ] Safari plays a stored MP4
- [ ] Task 324's 304 path re-measured, no regression
- [ ] OpenAPI updated (206, 416, q param)

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
