# 318 — GlimtViewer full-screen dialog and save-to-device

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §7, §0a.4. Tapping media opens a full-screen shadcn `dialog` with swipe between items.
**No auto-advance** — this is not a story tape.

**Saving.** "Going through photos" ends in "I want that one", so a *Gem* action is a
requirement, not a nicety. `showSaveFilePicker` is not on the iOS 16.4 baseline, so this is a
plain anchor `download` on the media URL, which on iOS Safari opens the share sheet and lets
"Save to Photos" through. **Verify on a real device** rather than assuming; long-press on the
image is the fallback iOS users already know.

Yes, this lets someone put a photo on Instagram afterwards. That is fine — the goal was never
to trap the photos, only to stop those platforms being the *only* place to put them.

Video is muted by default and never autoplays with sound.

## Acceptance Criteria

- [ ] `GlimtViewer.vue` full-screen dialog, swipe between items, no auto-advance
- [ ] *Gem* action downloads the current item
- [ ] Verified on iOS Safari 16.4+ that the save path reaches Photos (log the result)
- [ ] Video muted by default, no sound autoplay
- [ ] Keyboard and screen-reader navigable
- [ ] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
