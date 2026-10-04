# 501 — Patrol tagging for videos

**Status:** done
**Priority:** medium
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

Videos can be patrol-tagged like photographs (`photo_patrol`). Task 489's Start/Slut auto-filing skips videos.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Tagging UI and API accept videos
- [x] Auto-filing never files a video, with a test

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — Tagging needed no change: it addresses library items by id and a video is one; `TestAdminTagsAVideo` pins it. The contact sheet's existing 'Tag patrulje' action applies to video tiles as to photographs.
- 2026-10-04 — `syncDiplomaAlbum` skips a patrol 'photograph' whose content type is video or whose library row is a video, and its removal pass now takes out any video in Start/Slut, tagged or not, with the reason 'Videoer hører ikke til i dette album'; an untagged photograph a curator added still stays. `TestStartAndSlutHoldNoVideos`.
