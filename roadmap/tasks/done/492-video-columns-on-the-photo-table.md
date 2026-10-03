# 492 — Video columns on the photo table, and delete walks that know them

**Status:** done
**Priority:** high
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-03
**Completed:** 2026-10-03

## Description

Add `kind`, `status`, `durationMs`, `videoRef`, `videoSdRef` and `failReason` to the `photo` table (PRD 029 §8, option A). Existing rows default to `kind=photo`, `status=ready`. Every shared-blob delete walk (`RefsInUse` et al.) must claim the new refs.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Columns added; existing photographs unaffected
- [x] Delete walks claim `videoRef`/`videoSdRef`, with tests in the style of `medium_test.go`

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-03 — Picked up while task 491's transcode benchmark runs.
- 2026-10-03 — Columns added in table.sql and via EnsureColumn; `video_lookup`/`video_sd_lookup` keys on both paths. For a video, `blobRef` is the 720p MP4 and `thumbRef`/`mediumRef` the poster, so existing readers serve something playable; `videoRef` names the 720p explicitly for the delete walks.
- 2026-10-03 — `RefsInUse`, `Get`, `libraryColumns` and the admin takedown know `videoRef`/`videoSdRef`. Guard tests in `medium_test.go` extended; `TestAdminDeleteFreesEveryVideoRendition` added. Full suite green.
