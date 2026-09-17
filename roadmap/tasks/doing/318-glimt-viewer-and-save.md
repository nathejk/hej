# 318 — GlimtViewer full-screen dialog and save-to-device

**Status:** doing
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent
**Started:** 2026-09-17
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

- [x] `GlimtViewer.vue` full-screen dialog, swipe between items, no auto-advance
- [x] *Gem* action downloads the current item
- [ ] Verified on iOS Safari 16.4+ that the save path reaches Photos (log the result)
      — **needs a device.** This is the only criterion left and it cannot be met from here;
      see the progress log.
- [x] Video muted by default, no sound autoplay
- [x] Keyboard and screen-reader navigable
- [x] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 — Built `vue/src/components/glimt/GlimtViewer.vue`, plus
  `vue/src/composables/useOpenGlimt.ts` for the open state. Wired into the feed
  (`GlimtView.vue`) and the hold grid (task 326).

  Decisions worth keeping:

  - **`object-contain`, unlike the feed.** The feed and grid crop to a shared shape so cards do
    not change height; here there is nothing below to shove around, and this is the surface
    where a member checks *what they actually shared*, so nothing is cropped.
  - **Full media only here.** This is what makes the two-cache split (task 315) work — the feed
    and grid pull thumbnails by the thousand, and the ~30 kB original is fetched one at a time
    by somebody who chose to look closely.
  - **Stepping controls are always visible**, not `pointer-coarse:hidden` like the feed's. The
    feed's arrows duplicate a swipe on the card; this dialog has no swipe of its own, so on a
    phone these are the only way through. Index wraps, matching the feed strip.
  - **The index is re-seeded on `(glimt.id, ordinal)`**, so opening item 3 of one glimt does not
    leave the viewer on item 3 of the next.
  - **`showCloseButton` is off**; the default close sits inside padding this dialog does not
    have, so there is an explicit one in the header.
  - Download filename is `nathejk-{date}-{hold}-{n}.jpg` rather than the glimt's uuid, which
    means nothing to the person who now has the file among their own photographs.

  Extracting `useOpenGlimt` was not tidying: **two surfaces open the same viewer**, and
  duplicated state invites them to drift in the one way that matters — which item you land on.
  It is deliberately not a Pinia store, so a viewer left open cannot reappear over the next page.

- 2026-09-17 — **Blocked on hardware for the last criterion.** The save path is a plain anchor
  `download`, chosen because `showSaveFilePicker` is not on the iOS 16.4 baseline. On iOS Safari
  this is expected to open the share sheet, where "Save to Photos" lives. That expectation is
  **unverified and marked as such in the component**, because whether Safari honours `download`
  or navigates to the image is exactly the kind of thing that cannot be reasoned about from a
  test suite — Vitest here runs in node with no DOM. Long-press on the image remains the
  fallback iOS users already know, so the feature is not dependent on this working.

  Everything else is done and green. Left in `doing/` rather than closed so the device check is
  not quietly lost.
