# 502 — Videos in the album zip download

**Status:** done
**Priority:** low
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

The album zip includes videos: the original file for 'Original', the 720p MP4 for every other size. Streams without buffering whole files.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Both cases produce playable files
- [x] Memory stays flat — by construction (the entry is `io.Copy` from the store's file handle into a stored, uncompressed zip entry, as originals already are); not measured with a real 22-min clip

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — `writeAdminAlbumZipVideo`: a video is never re-rendered. 'Original' streams the phone's file under its own name; every other size streams the 720p MP4 with the entry's extension changed to `.mp4`. A video still processing (no MP4 yet) gets its original rather than leaving a gap in the numbered sequence. `TestTheAlbumZipCarriesVideos`.
