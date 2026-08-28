# 106 — Frontend: `PhotoCapture.vue`

**Status:** open
**Priority:** medium
**Created:** 2026-08-28

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

- [ ] Preview, face guide, shutter, flip, cancel, retake/confirm.
- [ ] Downscale + orientation fix before emitting the blob.
- [ ] `<input capture="user">` fallback.
- [ ] All tracks stopped on every exit path, including unmount.
- [ ] Component API is general enough for PRD 005's onboarding step (emits a
      blob; owns no upload logic).

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
