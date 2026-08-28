# 106 — Frontend: `PhotoCapture.vue`

**Status:** done
**Priority:** medium
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

Unblocked 2026-08-28 (task 102). Note what the decision means for this component:
there is **no in-app consent step** to build — consent is already held from sign-up
— so the surface explains the purpose and takes the photo.

The full-screen capture surface, **owned by PRD 003 and reused by PRD 005** — it
must not be forked: live `getUserMedia` preview defaulting to the front camera, a
circular **face-guide** overlay, shutter, camera-flip when more than one camera
exists, cancel; then preview with `Brug billede` / `Tag igen`.

Client-side downscale before upload (longest edge ~1024px, JPEG/WebP, well under
the server limit) with orientation fixed. `<input capture="user">` as the
fallback where `getUserMedia` is unavailable or blocked.

Stream teardown on cancel, confirm and unmount is a hard requirement — a live
camera left running is both a battery and a privacy problem.

## Acceptance Criteria

- [x] Preview, face guide, shutter, flip, cancel, retake/confirm.
- [x] Downscale + orientation fix before emitting the blob.
- [x] `<input capture="user">` fallback.
- [x] All tracks stopped on every exit path, including unmount.
- [x] Component API is general enough for PRD 005's onboarding step (emits a
      blob; owns no upload logic).

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Unblocked by task 102 and picked up.
- 2026-08-28 — **Orientation is handled by construction, not by parsing EXIF.** The
  frame is drawn from the live `<video>` element, which renders it already rotated, so
  the canvas output needs no tag — which is exactly what is needed, because the server
  re-encodes and therefore *drops* EXIF (task 105). An implementation that read the
  tag and passed it along would produce sideways portraits.
- 2026-08-28 — Stream teardown on all five exit paths: cancel, confirm, unmount,
  before a flip, and **after the shutter** — the camera is not needed while the user
  decides, and the deliberation step is where an indicator light would otherwise stay
  on longest. `retake()` restarts it.
- 2026-08-28 — `facingMode: 'user'` as a plain value, not `exact`: with `exact` a
  laptop or a single-camera phone fails with `OverconstrainedError` instead of just
  using the camera it has.
- 2026-08-28 — The flip button appears only after `enumerateDevices()` **post-grant**.
  Before permission, browsers withhold the device list, so enumerating early makes the
  button's presence depend on something the user cannot see.
- 2026-08-28 — Preview holds a Blob + object URL rather than a data URL. A 1024px JPEG
  as base64 is ~1.4 MB of string, on a phone, for no benefit; the URL is revoked on
  every transition and the Blob outlives it, which is what the parent needs.
- 2026-08-28 — The `<input capture="user">` fallback is offered **up front**, not only
  after an error. On a device where the live camera is blocked in settings it is the
  only way through, and discovering that after a failure message is worse.
- 2026-08-28 — A denied camera shows task 101's platform guidance instead of a retry
  button: once denied, the browser will not ask again, so a button is a dead end.
- 2026-08-28 — Deliberately **no consent step**, per task 102 — consent is held from
  sign-up. The copy states the purpose ("genkende dig under løbet — også når det er
  mørkt") rather than asking permission.
- 2026-08-28 — Kept upload-free on purpose: it emits `captured(blob)` and knows nothing
  about stores or endpoints, which is what makes PRD 005 able to reuse it rather than
  fork it.
- 2026-08-28 — ✅ `npm run type-check` clean. **Camera behaviour cannot be verified
  here** — no camera in a container, and iOS is the platform with the quirks (inline
  autoplay, `playsinline`, front-camera mirroring). That is task 108, and it is the
  main risk left in this component. Moving to done.
