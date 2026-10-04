# 495 — Transcode worker

**Status:** done
**Priority:** high
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

A dedicated goroutine in the API binary, one job at a time, `nice`d. It produces the 720p MP4, the 480p MP4 for clips over 5 min, and posters at the thumb and medium sizes (cache class). Jobs are idempotent per original ref and survive restart; a rendition cache miss re-queues the job. Emits `video.transcoded`/`video.failed`.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Renditions match the PRD 029 §6 settings (faststart, metadata stripped, rotation applied)
- [x] Restart mid-job resumes
- [x] Cache miss rebuilds — mechanism (`requeueVideo`) here; the serving path that detects the miss calls it in task 496
- [x] Retry endpoint for failed items, with OpenAPI annotations

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — `internal/video/transcode.go`: 720p (CRF 23, maxrate 2.5M, AAC 128k, `-map_metadata -1`, `+faststart`, no upscaling, short edge 720 so portrait clips stay portrait), 480p from the 720p for clips > 5 min, poster frame at 1 s. Run under `nice -n 10`. Exit-status failures are `ErrTranscode` (the file's fault); anything else is the server's.
- 2026-10-04 — `cmd/api/videoworker.go`: one goroutine started from main, woken by uploads/retries, polling each minute. The projection is the queue (`PhotoCurator.ProcessingVideos`, across years), so a restart resumes; a 15-min `recent` set covers projection lag and backs off server-side failures without marking the clip failed. Renditions go to the cache class via `PutFile`; the poster goes through `imaging.Prepare` for the usual thumb/medium sizes. A missing original publishes `videofailed`.
- 2026-10-04 — Retry is `POST /api/admin/videos/retry/{photoId}` (failed only; 409 otherwise; 202 + `videoqueued`). The path is not `/videos/{id}/retry` or `/photos/{id}/retry` because httprouter refuses a wildcard beside the static `uploads` and `renditions` segments. Worker file added to the curator and original boundary allow-lists with reasons. Tests with a fake ffmpeg; full suite and staticcheck green.
