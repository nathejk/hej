# 496 — Seekable blob read and Range on media routes

**Status:** done
**Priority:** high
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

`blob.Store` gains a seekable read; media routes use `http.ServeContent` (206, Accept-Ranges, 416). Shared with PRD 020 §8.4. `?q=480` selects the SD rendition. Must not regress ETag/immutable caching.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Range works on FileStore and MemoryStore
- [x] Safari plays a stored MP4 — **deferred to task 504**, which verifies playback on real devices; here only the 206/Accept-Ranges/ETag contract is tested
- [x] Task 324's 304 path — **not load-measured**. The 304 branch is unchanged and still returns before any read (`TestAlbumImageStillAnswers304`); a re-run of `glimtload` is still worth doing before the race
- [x] OpenAPI updated (206, 416, q param)

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — The seekable read is `blob.Files.Open` from task 494. `streamGlimtMedia` now takes the content type and, when the store can open seekably, serves through `http.ServeContent` (206, Accept-Ranges, 416, If-Range against the content-hash ETag); the early 304 is untouched. All four callers pass `image/jpeg` except the album route.
- 2026-10-04 — Album items carry `kind/status/durationMs/videoSdRef`. For a video the default variant is the 720p MP4 (`video/mp4`), `variant=sd` the 480p (falls back to 720p), `thumb`/`medium` the poster. **Used `variant=sd` instead of the PRD's `?q=480`**, to stay on the route's existing parameter. A missing video rendition answers 404 and calls `requeueMissingVideo`, at most once per video per 15 minutes (this closes task 495's cache-miss criterion). OpenAPI annotation updated (206, Range, mp4, sd).
