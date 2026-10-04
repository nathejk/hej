# 491 — ffmpeg and ffprobe in the API image

**Status:** doing
**Priority:** high
**Created:** 2026-10-03
**Picked up by:** claude (agent session)
**Started:** 2026-10-03
**Completed:**

## Description

Add ffmpeg/ffprobe to the API Docker image. Measure the image size growth and the time to transcode a 22-min 2026 clip on the production host with the PRD 029 §8.1 settings.

Part of PRD 029 (Video in public albums).

## Acceptance Criteria

- [x] ffmpeg and ffprobe available in the API image
- [x] Image size delta recorded
- [ ] 22-min transcode time on the prod host recorded (target ≤ 2× real time)

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-03 — Task created from PRD 029.
- 2026-10-03 — Picked up.
- 2026-10-03 — Compared install options on alpine:3.20: `apk add ffmpeg` (6.1.1, has libx264 and aac) takes the image from 9.9 MB to 121 MB, i.e. **+111 MB**. The static build (`mwader/static-ffmpeg:7.1`) is 104 MB per binary, **+208 MB** for ffmpeg+ffprobe. Chose apk for prod, and Debian's `ffmpeg` in `api-dev` (a different version, so the worker sticks to options both support).
- 2026-10-04 — **Blocked on the timing measurement.** A first attempt generated a 22-min synthetic 1080p source with a per-frame noise filter: 8.7 GB, failed to mux, killed at the time limit. A second, 2-min attempt hung, and the local Docker daemon then stopped responding (likely its VM disk filled by the first attempt). The criterion asks for the production host anyway: run `nice -n 10 ffmpeg -i <a real 2026 clip> -map_metadata -1 -vf "scale=..." -c:v libx264 -preset medium -crf 23 -maxrate 2.5M -bufsize 5M -pix_fmt yuv420p -c:a aac -b:a 128k -movflags +faststart out.mp4` there and record the `speed=` figure. The Dockerfile change is committed; the rest of PRD 029 does not depend on the number, only the preset choice might.
