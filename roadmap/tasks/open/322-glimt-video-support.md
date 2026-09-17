# 322 — Video support: 30 s cap, container validation, metadata verification

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] Accepted container/codec set documented and validated on upload
- [ ] Server-side duration cap enforced on the parsed container; 400/413 past it
- [ ] 50 MB per-item ceiling enforced
- [ ] **Verified on real iOS and Android recordings** that no location metadata survives — result recorded in this log
- [ ] Client-side re-encode/compression where supported
- [ ] Video renders muted, no sound autoplay
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
