# 494 — Chunked, resumable admin video upload

**Status:** done
**Priority:** high
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

Admin upload sessions: create, PUT chunks with Content-Range, GET offset, complete (PRD 029 §8.4). Caps: ≤ 30 min (ffprobe) and ≤ 4 GB. The original is stored byte for byte in the backed-up class. Incomplete uploads are cleaned up after 24 h.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Upload resumes after interruption
- [x] 413 over 4 GB; refusal over 30 min names the limit
- [x] Original stored and `video.uploaded` emitted
- [x] OpenAPI annotations for every new endpoint

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — `blob.Files` added as an optional capability (like `FreeSpacer`) on both stores: `StagingDir` (same volume, outside original/ and cache/), `PutFile` (streamed hash + rename, same dedup/promotion rules as Put), `Open` (seekable, for task 496) and `LocalPath` (for ffmpeg). `internal/video` wraps ffprobe (duration, size, rotation, audio, cover-art rejection).
- 2026-10-04 — Endpoints in `cmd/api/adminvideoupload.go`: create (size ≤ 4 GiB, disk floor at 2× size, sweeps sessions idle > 24 h), GET offset, PUT chunk (Content-Range must start at the offset, else 409 with the offset; a short chunk is truncated back off), complete (ffprobe: must be video, ≤ 30 min → 413 naming the limit; hashed before adoption so a previously deleted clip is not put back on disk; publishes `videouploaded`). Session state is files only, so a restart loses no upload. OpenAPI annotations added and the failure-code guard passes; file added to the curator boundary allow-list.
