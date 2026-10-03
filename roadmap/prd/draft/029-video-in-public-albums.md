# PRD 029 — Video in public albums

**Status:** draft
**Author:** agent session
**Created:** 2026-10-03
**Last updated:** 2026-10-03
**Approved:**
**Shipped:**
**Target users:** photographer (uploads, via the PRD 022 library) · public visitor (watches, on the public site)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Let a photographer add **video clips** to the photo library and to albums, next to photographs. The
server transcodes every clip to a single **720p H.264 MP4** with a poster frame. Clips longer than five
minutes also get a **480p** version. In the album viewer the current video **autoplays muted**, and
**Space** toggles pause.

## 2. Problem & Motivation

**What problem does this solve?** The photographers film as well as photograph. Most clips are under
three minutes, but at least one 2026 clip ran **22 minutes**. Today the library takes only images, so
the clips stay on someone's phone or end up on another platform, outside the albums where the race is
remembered.

**Why now?** PRD 022's library, PRD 023's viewer and PRD 027's originals give this a place to land.
The originals story is settled, so a video original can follow the same rules.

**How this differs from PRD 020 (Video in Glimt).** PRD 020 is participant video: 30 seconds, no
transcoding, and gated on a metadata-stripping container rewrite. This PRD is **crew-uploaded,
curated, long-form** video. The two have different constraints:

| | PRD 020 (glimt) | PRD 029 (albums) |
|---|---|---|
| Who uploads | any participant, from a phone, on event data | a photographer, after the race, usually on wifi |
| Length | ≤ 30 s | up to ~30 min |
| Transcoding | ruled out | **required**: phone HEVC `.mov` does not play in Firefox or most Chromium |
| Metadata | container rewrite, verified on devices | dropped by re-encoding (`-map_metadata -1`); the original is kept as PRD 027 keeps photo originals |

The transcoding pipeline here runs out-of-band, in a worker. If PRD 020's verification gate fails and
glimt needs `ffmpeg` after all (PRD 020 §4), it can reuse this pipeline.

**Evidence.** Maintainer request, 2026-10-02/03: video in public albums, autoplay, Space to pause. The
2026 race produced clips of up to 22 minutes.

## 3. Goals

- A photographer can upload a clip (up to 30 min) to the library and place it in an album.
- Every visitor can play it in any current browser, including iOS Safari, Firefox and the installed
  PWA, without plugins or a streaming library.
- A long clip plays acceptably on mediocre mobile data.
- Album pages stay as light as they are today. A video costs a poster image in the grid, not megabytes.
- The original is kept and backed up. Every rendition can be rebuilt from it.

## 4. Non-Goals

- **Adaptive streaming (HLS/DASH).** Out of scope for now; see §8.2 for why, and for when to revisit.
  Because originals are kept, it can be added later without re-uploading.
- **Editing.** No trimming, cutting or rotation. Upload a finished clip.
- **Participant upload.** That is PRD 020.
- **Patrol tagging and auto-filing of video** (task 489's Start/Slut albums). Photos only for now.
- **Video in the zip download.** The album zip stays photographs; see §11.
- **Subtitles/captions.** Not now.

## 5. User Stories & Scenarios

- As a **photographer**, I want to drop a folder containing both photos and clips into the library, so
  that I do not have to sort them first.
- As a **photographer**, I want to see that a clip is still processing, so that I know why it does
  not play yet.
- As a **visitor**, I want a clip to start playing when I swipe to it, so that the album flows like
  the race did.
- As a **visitor**, I want to press Space to pause, and use ← / → to seek, so that I can watch a long
  clip properly.
- As a **parent on 4G**, I want a 22-minute clip to play without stalling.

### Primary path

1. A photographer drags files into the library (PRD 022). Video files are recognised and uploaded in
   resumable chunks (§8.4).
2. The server stores the upload as the **original** (backed-up class) and emits `video.uploaded`. The
   library shows the item with a "behandles…" placeholder.
3. A worker runs ffmpeg. It produces the 720p MP4, a 480p MP4 if the clip is longer than 5 minutes,
   and a poster JPEG in the existing thumb and medium sizes. It then emits `video.transcoded` with the
   rendition refs, duration and dimensions.
4. The photographer adds the clip to an album like a photograph.
5. A visitor opens the album. The grid shows the poster, a play glyph and the duration.
6. The visitor opens the clip in the viewer. It autoplays **muted**, with an unmute button. Space
   toggles pause. Swiping to the next item pauses and unloads the video.

### Edge cases

| case | behaviour |
|---|---|
| Transcode fails (corrupt or unsupported file) | `video.failed` with the reason. The library shows the error, and the item cannot be placed in a public album. The original is kept so a retry is possible. |
| Worker or server restart mid-transcode | The job is picked up again. Jobs are idempotent and keyed on the original's ref. |
| Upload interrupted | Resume from the last acknowledged chunk. An incomplete upload is never referenced and is cleaned up after 24 h. |
| Clip over the length or size cap | Refused at upload with a message naming the limit. Duration is checked again by ffprobe server-side. |
| Re-upload of the same file | Lands on the same item, as photos do (content-hash id). No second transcode. |
| Browser refuses autoplay even muted (Low Power Mode, data saver) | The poster is shown with a large play button. |
| Portrait (vertical) phone video | Rotation is applied, then the clip is scaled so its short edge is 720 (720×1280). The viewer letterboxes it. |
| Clip with no audio track | Plays; the unmute button is hidden. |

## 6. Requirements

### Functional

- [ ] The library accepts video files (`video/mp4`, `video/quicktime`, `video/webm`, and whatever
      ffprobe identifies as a video stream). Detection uses ffprobe, not the extension.
- [ ] Caps: **≤ 30 min duration, ≤ 4 GB per original**, enforced server-side.
- [ ] Uploads are **chunked and resumable**.
- [ ] The original is stored byte for byte in the backed-up blob class (PRD 027 rules).
- [ ] Renditions go in the **cache** class (task 429 rules): `video720`, `video480` (only if > 5 min),
      `posterThumb`, `posterMedium`.
- [ ] Transcode settings: H.264 High, `yuv420p`, CRF 23, `maxrate` 2.5 Mbit/s (720p) / 1 Mbit/s
      (480p), AAC 128k (96k at 480p), `+faststart`, `-map_metadata -1`, rotation applied.
- [ ] Transcoding is async in a worker. Events: `video.uploaded`, `video.transcoded`, `video.failed`
      on the PHOTO stream.
- [ ] Only items whose transcode has succeeded can appear in a **published** album.
- [ ] Every media route that serves video honours **HTTP Range** (206, `Accept-Ranges: bytes`)
      without breaking the existing ETag/`immutable` caching.
- [ ] Viewer: `autoplay muted playsinline`, a visible unmute button, and native or custom controls
      with a seek bar and duration.
- [ ] Keyboard: **Space** toggles play/pause (and `preventDefault`s scroll), **← / →** seek ±10 s,
      **M** toggles mute. Ignored while focus is in a form field.
- [ ] Only the visible item plays. Leaving it pauses it and releases the source.
- [ ] Grid: poster + play glyph + duration; `preload="none"` outside the viewer.
- [ ] The viewer picks **480p** for clips that have one when `navigator.connection.effectiveType` is
      `3g` or lower, or after a stall of more than 2 s. A manual "HD" toggle is always available.
- [ ] Social share previews (PRD 026) of a video item use the poster.
- [ ] Deleting a video removes the original and all renditions. **Every shared-blob delete walk
      (`RefsInUse` et al.) learns the new ref columns** (the lesson from task 409).

### Non-Functional

- **Compatibility.** Plays in current Safari (macOS/iOS, incl. installed PWA), Chrome, Firefox, Edge
  and Android Chrome.
- **Bandwidth.** 720p ≈ 15–20 MB per minute, 480p ≈ 7–8 MB per minute. Nothing downloads before
  the viewer opens.
- **Transcode time.** A 22-min clip completes within ~2× real time on the production host; measure it.
  Encoding runs at reduced priority (`nice`) so it never starves the API.
- **Storage.** The originals dominate: a 22-min 4K iPhone clip can be 3+ GB. See §11 Q1.
- **Accessibility.** Controls ≥ 44 px, keyboard operable, focus visible. Autoplay is always muted,
  and nothing plays sound without an explicit action.

## 7. UX / UI Notes

- **Library (PRD 022 admin).** A video item looks like a photo tile with a duration badge. While
  processing it shows a spinner tile with "behandles…". When failed it shows a red tile with the reason
  and a "prøv igen" button.
- **Album grid (public).** Poster image, play glyph, duration (`m:ss`, or `h:mm:ss`).
- **Viewer (PRD 023).** Same frame as photos, with a `<video>` instead of `<img>`. Controls overlay at
  the bottom: play/pause, scrubber, time, mute, HD/SD. A short first-view hint reads
  "Mellemrum = pause".
- **Swiping vs. scrubbing.** Horizontal swipes on the scrubber must seek, not change item.

## 8. Technical Considerations

### 8.1 Pipeline

- **BFF (Go):** extend `POST /api/admin/photos` (or add a sibling upload route) for chunked upload.
  When the upload completes, store the original and emit `video.uploaded`.
- **Worker:** a goroutine pool in the API process, or a separate binary in the same image. It consumes
  `video.uploaded`, shells out to `ffprobe`/`ffmpeg`, writes renditions to the blob store and emits
  `video.transcoded`/`video.failed`. **`ffmpeg` is added to the API Docker image** (a deliberate
  change from PRD 020's stance, which applies only to the glimt path).
- **Rebuild on cache miss:** as task 430 rebuilds photo renditions, a missing video rendition
  re-queues a transcode. Until it completes, the item serves from the other rendition if one exists,
  otherwise it shows "behandles…".

Reference command (720p):

```
ffmpeg -i in -map_metadata -1 -vf "scale='if(gt(iw,ih),-2,720)':'if(gt(iw,ih),720,-2)'" \
  -c:v libx264 -preset medium -crf 23 -maxrate 2.5M -bufsize 5M -pix_fmt yuv420p \
  -c:a aac -b:a 128k -movflags +faststart out.mp4
```

### 8.2 Why no HLS

A faststart MP4 with Range support starts playing at once, seeks anywhere and downloads only what is
watched. HLS would add segments, manifests, hls.js outside Safari and many more blobs for the delete
walks to track. Its benefit is mid-stream quality switching, and the 480p rendition plus the client
heuristic gets most of that for long clips. **Revisit if** long clips become common, or if stall
metrics (§9) show real pain.

### 8.3 Range serving

`blob.Store.Get` returns `io.ReadCloser`. Video needs `io.ReadSeeker` (or a `GetRange`) so the media
routes can use `http.ServeContent`. That is two implementations (`FileStore`, `MemoryStore`). The
same change is listed in PRD 020 §8.4; **whichever PRD lands first does it**. Re-measure the
304/ETag path (task 324).

### 8.4 Chunked upload

A simple protocol: `POST` creates an upload session and gets an id. `PUT` sends chunks with
`Content-Range` (≈ 8 MB each). `GET` returns the received offset so the client can resume, and a
final `POST` completes. tus.io is an alternative if a library is preferred. Admin only, behind the
existing admin auth.

### API endpoints

- `POST /api/admin/videos/uploads`, `PUT/GET /api/admin/videos/uploads/{id}`,
  `POST /api/admin/videos/uploads/{id}/complete` (new).
- `POST /api/admin/videos/{id}/retry` (new).
- `GET /api/public/albums/{id}/media/{ordinal}` (changed): serves `video/mp4`, honours Range (206),
  and takes `?q=480` for the SD rendition.
- Album and library list responses (changed): gain `kind`, `durationMs`, `status`, `hasSd`.

**Every new or changed endpoint needs OpenAPI annotations** (`.rules`), including 206, 413 and 416.

### Data / storage

- **Option A: extend the `photo` table** with `kind` (`photo`|`video`), `status`
  (`processing`|`ready`|`failed`), `durationMs`, `videoRef`, `videoSdRef`, `failReason`. The poster
  uses the existing `thumbRef`/`mediumRef` and the original uses the PRD 027 column. This keeps albums,
  ordering (PRD 024) and credit (PRD 025) unchanged. **Recommended.**
- Option B: a separate `video` table, with album items pointing at either. Cleaner names, but every
  album query gets a union.
- Either way, the **shared-blob delete walks must claim `videoRef` and `videoSdRef`**, with tests
  like `medium_test.go`.

### Dependencies & risks

| risk | mitigation |
|---|---|
| ffmpeg in the image grows it (~80–100 MB) | Accept, or run the worker as a separate image. |
| CPU-heavy encode slows the API during the race | `nice`, one concurrent job, and in practice uploads happen after the race. |
| Disk fills from originals | Decide §11 Q1 before shipping. Add a total-storage check on upload. |
| Delete walk does not know a new ref → orphaned or wrongly deleted bytes | Explicit tests, as in task 409. |
| Range change regresses image caching | Re-measure task 324's numbers. |
| HEVC/HDR input yields washed-out colour | Tone-map HDR (`zscale`/`tonemap`) or accept for v1; check with a real iPhone HDR clip. |

## 9. Success Metrics

- Every uploaded clip from the 2026 race (including the 22-min one) transcodes and plays on iOS
  Safari, Android Chrome and desktop Firefox. Pass/fail.
- Time from upload complete to ready: under 2× clip duration (p95).
- Playback start under 2 s on 4G (poster shown at once).
- Rebuffer ratio under 2% on 720p, wifi; long clips on 3G complete using 480p.
- No regression in album page weight or task 324's p95.

## 10. Rollout / Task Breakdown

- **Phase 1:** pipeline and admin (upload, transcode, library tiles). No public exposure.
- **Phase 2:** Range serving, public viewer, grid, keyboard.
- **Phase 3:** SD rendition and the client heuristic, measured with a real long clip on throttled
  network.

Proposed tasks for `roadmap/tasks/open/` (created on approval):

- [ ] Task: Add ffmpeg/ffprobe to the API image; measure image size and a 22-min transcode on the prod host
- [ ] Task: Schema: `kind`, `status`, `durationMs`, `videoRef`, `videoSdRef`, `failReason`, plus delete-walk tests
- [ ] Task: Video events (`uploaded`/`transcoded`/`failed`) on the PHOTO stream and their folds
- [ ] Task: Chunked, resumable admin upload with caps and OpenAPI annotations
- [ ] Task: Transcode worker: idempotent jobs, 720p, 480p for >5 min, poster renditions, retry, rebuild on cache miss
- [ ] Task: `blob.Store` seekable read and Range on media routes (shared with PRD 020), re-measuring the 304 path
- [ ] Task: Library UI: video tiles, processing/failed states, retry
- [ ] Task: Public grid: poster, play glyph, duration
- [ ] Task: Viewer: muted autoplay, unmute, controls, Space/←/→/M, pause on leave
- [ ] Task: SD selection heuristic and HD toggle
- [ ] Task: Share previews and the publish gate (only `ready` video in published albums)
- [ ] Task: Verify the 2026 clips end to end on iOS, Android and Firefox

## 11. Open Questions

1. **Keep 4K originals forever?** They are the bulk of the storage. Alternatives: keep a 1080p
   high-quality master (CRF ~18) instead, or keep originals for N months. PRD 027's reasoning favours
   keeping, but the volume has to be sized for it.
2. **Caps:** are 30 min and 4 GB right?
3. **Autoplay with sound** when the viewer was opened by a click? Browsers usually allow it after a
   user gesture. The default here is muted, always.
4. **Loop short clips** (e.g. under 15 s), like a live photo? The default is no loop.
5. **Zip download:** include the 720p MP4s, skip videos, or make it an option?
6. **Worker in-process or as its own binary/container?** In-process is simpler; separate isolates the
   CPU load.
7. **Do videos join patrol tagging and auto-filing (task 489)** later?
