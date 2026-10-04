# 493 — Video events on the PHOTO stream and their folds

**Status:** done
**Priority:** high
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-03
**Completed:** 2026-10-03

## Description

`video.uploaded`, `video.transcoded` and `video.failed` on the PHOTO stream, folded into the `photo` table. Refs are validated as photo refs are.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Events defined and published on the PHOTO stream
- [x] Folds write status/refs/duration; invalid refs blanked
- [x] Re-upload of the same original lands on the same row

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-03 — Picked up.
- 2026-10-03 — Verbs `videouploaded`, `videotranscoded`, `videofailed` plus `videoqueued` (retry and cache-miss rebuild, needed by task 495) in `photo/videoevents.go`. A video's id is the hash of the **original**, since no display rendition exists at upload; a re-upload touches only `fileName`. Transcoded/failed/queued are UPDATEs scoped to `kind="video"`; failed only applies to a `processing` row so a failed rebuild cannot unpublish a ready video. Tests in `videoevents_test.go`; the verb guard covers the new verbs.
