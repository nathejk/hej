# 503 — Share previews and the publish gate for video

**Status:** done
**Priority:** medium
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

PRD 026 share previews use the poster for video items. Only `status=ready` videos appear in published albums.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Share card shows the poster
- [x] Processing or failed videos never visible publicly, with a test

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — Gate: every public album read in `album/querier.go` (item count, cover choice, items) requires `p.status = "ready"`; the curator reads do not, so a curator can still see and retry a failed clip. Photographs are always ready. Source guard `TestPublicReadsServeOnlyReadyItems`.
- 2026-10-04 — Share cards needed no new code: the permalink and album cards already ask for `?variant=medium`, which for a video is its poster. Found and fixed a real hole while testing it: a video with no medium poster fell back to `Ref` — the MP4 — served as `image/jpeg`. A poster request now uses the other poster size, or answers 404, and never the MP4 (`TestAVideoPosterNeverFallsBackToTheMP4`, `TestAVideoShareCardIsItsPoster`).
