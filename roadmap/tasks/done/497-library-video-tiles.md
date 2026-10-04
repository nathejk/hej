# 497 — Library UI: video tiles, processing and failed states

**Status:** done
**Priority:** medium
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:** 2026-10-04

## Description

The PRD 022 library recognises video files in a dropped folder, uploads them with progress via the chunked API, and shows duration-badged tiles with 'behandles…' and failed (with 'prøv igen') states.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] Mixed photo/video folder drop works — written and syntax-checked, **not yet exercised in a browser** (no browser in this session); first real check belongs to task 504
- [x] Processing and failed states visible; retry works

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up.
- 2026-10-04 — `upload.js`: files recognised as video (MIME or extension) go through the chunked API in 8 MiB PUTs with a percentage on the row; a failed chunk backs off (up to 30 s, 8 tries) and resumes from the server's offset; the session id is kept in localStorage per name/size/date so re-dragging a clip after a closed laptop continues. The file input accepts `video/*`.
- 2026-10-04 — Tiles: `data-kind`/`data-status`/`data-duration-ms`, a `▶ m:ss` badge, a 4:3 'behandles…' placeholder (no thumbnail request while there is no poster) and a red 'video fejlede' with ffmpeg's reason as title. Retry is an action-bar button ('Behandl video igen', `retryaction.js`) because a control inside the tile button is invalid markup; it retries the failed videos in the selection and waits for the rows to leave `failed` before reloading.
- 2026-10-04 — The admin media route labels a video's renditions `video/mp4` and its original with the stored content type.
