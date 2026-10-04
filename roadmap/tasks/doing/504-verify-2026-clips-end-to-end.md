# 504 — Verify the 2026 clips end to end

**Status:** doing
**Priority:** high
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-04
**Completed:**

## Description

Upload the 2026 race clips (including the 22-min one) and verify playback, seeking, sound rules and SD switching on iOS Safari (and installed PWA), Android Chrome and desktop Firefox. Record the results against PRD 029 §9.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [ ] All clips transcode — one so far: 'Bag om startposten på Nathejk 2026.mp4'
- [ ] Playback verified on all three platforms
- [ ] Results recorded in the progress log

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-04 — Picked up with the one 2026 clip on hand: 'Bag om startposten på Nathejk 2026.mp4' (38.25 s, 1080×1080 H.264 30 fps, AAC, 19 MB).
- 2026-10-04 — `internal/video/sample_test.go` (opt-in, `VIDEO_SAMPLE=`) ran the real Probe + Transcode in the api-dev image (Debian ffmpeg): 720p = 720×720, 10.3 MB/min, 3.6× real time; with 480p forced, 480×480 at 5.2 MB/min and 2.1× real time for both. Audio kept, moov before mdat, no location atoms, valid JPEG poster. Prod's Alpine ffmpeg 6.1.1 accepts the identical command line: 8.9 s wall, 47 s CPU on 8 cores for 38 s of video (≈1.2 CPU-s per video-s; a 22-min clip is then ≈ 27 CPU-min).
- 2026-10-04 — Full path in the dev stack (api rebuilt with ffmpeg): chunked upload over HTTPS through Traefik, a deliberately repeated chunk, complete, worker, projection, published album, public media route. 206/`video/mp4`/Accept-Ranges/immutable; poster 720×720 JPEG; the frame chosen is sensible.
- 2026-10-04 — **Three bugs found and fixed** in this run: (1) the 409 for an out-of-order chunk was sent without reading the 8 MiB body, so Go closed the connection and the client saw a broken pipe instead — the body is now drained first; (2) the upload's wake reached the worker before the projection had the row, so the job sat 56 s for the minute's poll — a wake now rechecks every second for 10 s (ready in 11 s after the fix, from 60); (3) in the viewer, Space pressed before playback began rejected `play()` with AbortError, which the autoplay fallback took for a refusal and restarted the video muted — it now retries only on NotAllowedError for the same item.
- 2026-10-04 — `go/scripts/viewer-smoke/smoke.mjs <mp4> <poster>`: the viewer checks, kept in the repo this time (the /tmp harness was lost with a reboot). 16/16 on the server's own output, three runs.
- 2026-10-04 — **Remaining, needs a person:** iOS Safari (and the installed PWA), Android Chrome and desktop Firefox on real devices, and the other 2026 clips — none besides this one were found on this machine. Note for whoever tests in dev: Traefik routes `/2026/...` to the Vite UI, so the public album page is only reachable on the api container directly (`localhost:4000` inside it); the dev DB now holds a published test album 'Videotest PRD 029'.
- 2026-10-04 — Maintainer tested in Safari: all fine. Still open: iOS/installed PWA, Android Chrome, Firefox, and the other 2026 clips.
