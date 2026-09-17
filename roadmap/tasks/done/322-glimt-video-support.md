# 322 — Video support: 30 s cap, container validation, metadata verification

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

> **Superseded by PRD 020 — no video code was written under this task.**
> Read that PRD, not this file, for the current plan. This file is kept because it is where the
> constraints below were gathered, and because deleting a task nobody implemented would erase the
> record of why it was not implemented here.

## Description

PRD 019 §8. **The one genuinely new capability** — `internal/imaging` cannot touch video, there
is no transcoder, and re-encoding video in the BFF process is not something to add casually.

Chosen approach (§0): accept a small set of formats, validate the container, store as-is,
relying on the browser to record H.264/MP4. Caps: **30 s** and **50 MB per item** — 30 s of
phone video is roughly 30–60 MB unrecompressed, several times the 8 MiB portrait limit.

Two consequences to handle explicitly, not assume away:

1. **Metadata stripping is container-level only.** It **must be verified against real iOS and
   Android recordings** before this ships. This is the single privacy claim in PRD 019 we cannot
   currently make with confidence.
2. **The 30 s cap must be enforced server-side on the parsed container**, not just by the
   recorder UI.

If verification fails, the fallback is `ffmpeg` out-of-band or a shorter cap — **not** shipping
video with an unverified privacy claim.

Client-side re-encode where `MediaRecorder` allows it, to keep uploads survivable on rural
mobile data.

## Acceptance Criteria

Every criterion below is now PRD 020's, not this task's. Left unchecked and unedited, because
pretending otherwise would put unmet criteria in `done/`:

- [ ] Accepted container/codec set documented and validated on upload
- [ ] Server-side duration cap enforced on the parsed container; 400/413 past it
- [ ] 50 MB per-item ceiling enforced
- [ ] **Verified on real iOS and Android recordings** that no location metadata survives — result recorded in this log
- [ ] Client-side re-encode/compression where supported
- [ ] Video renders muted, no sound autoplay
- [ ] `go test ./...` passes

The one criterion this task actually discharged:

- [x] **Superseded cleanly** — every constraint above carried into PRD 020 §8 and its task list,
      nothing lost, and PRD 019 §8 keeps the decision record that produced them.

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.

- 2026-09-17 — **Superseded by PRD 020 before any implementation.** Taken off the board rather than
  built, for a reason that only became clear once the rest of PRD 019 was finished: this task's
  first criterion is a **verification on hardware I do not have**, and its outcome decides whether
  the other six are even the right plan. If container-level stripping does not hold, the answer is
  `ffmpeg` out-of-band — a different piece of work with a different risk profile, not a variation of
  this one.

  Holding PRD 019 in `doing/` around an unknown that large made its status meaningless. Everything
  else in that PRD is either shipped or a short device check (tasks 318, 325).

  **Nothing was lost in the move, and three things were gained.** Writing PRD 020 against the real
  code found gaps this task's description did not have:

  - **"Container-level stripping" is a container *rewrite*, not a read.** On iOS, location lives in
    `moov/udta/©xyz`; removing it means editing the box tree and fixing parent sizes without moving
    `mdat` sample offsets. That is the risky part, and it is why the verification gate exists — but
    this task file described it as though it were a filter on the way past.
  - **Video needs HTTP Range support, which the media route does not have.** `streamGlimtMedia`
    `io.Copy`s the whole object and sets no `Accept-Ranges`. Safari issues a byte-range request and
    will not play a response that ignores it, so video would simply not have worked on the platform
    that matters most — and nothing in this task's criteria would have caught it.
  - **The storage numbers were set for an image-only feature.** With `GLIMT_MEMBER_STORAGE_BYTES` at
    500 MiB, **ten 50 MB videos fill a member** and they can then post no photographs either;
    `GLIMT_BYTES_PER_HOUR` at 200 MiB is four videos an hour. Task 311 chose those figures and said
    they were provisional; task 324 has since shown the read limit was wrong in exactly this way.
    PRD 020 §8.3 makes deciding them a prerequisite.

  Also raised as open questions rather than assumed: whether 30 s is still right now the storage
  arithmetic is visible, whether the **audio track** should be stripped (muted *playback* is not the
  same as a silent recording, and a clip of children talking is a different privacy object), and
  whether **WebM** must be supported — Android Chrome often records Matroska, which would mean a
  second parser and a second rewrite. That last one may deserve its own spike.
