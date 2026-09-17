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

- 2026-09-17 — **Tested on an iPhone. Both halves failed, and the second one was not a Glimt bug at
  all.**

  1. **The save icon was under the status bar**, overlapping the clock. The viewer is an edge-to-edge
     black surface and its header had no safe-area padding, so it sat inside the notch inset
     (`--sat` = 59px on this device). Fixed with `padding-top: calc(var(--sat) + 0.5rem)` on the
     header — `var(--sat)` rather than `env(safe-area-inset-top)`, per the rule in `main.css`.

  2. **Tapping it downloaded a 14 kB file called `nathejk-2026-09-17-team-1.jpg.html`.** The filename
     is the whole diagnosis: iOS appended `.html` because the response *was* HTML. Safari treats an
     `<a download>` click as a **navigation**, the service worker's navigation fallback answered it
     with `index.html`, and the member got the app shell renamed as a photograph.

     `navigateFallbackDenylist` covered `/desktop.html` and `/offentligt/` but **not `/api/`**. This
     is the second bug from that one array — the first was the public page (task 323) — and both
     failed the same asymmetric way: never for a developer in a browser tab, always for an installed
     member. `/api/` is now denied the fallback, which is right on its own merits: a browser that
     navigates to an API path should get the API's answer, bytes or a JSON 404, never the shell.

- 2026-09-17 — **The anchor was the wrong mechanism regardless, so it is gone.** Even with the
  fallback fixed, `download` on iOS routes through the download manager into **Files**, not Photos —
  so the criterion as written could not have been met by an anchor at all. The camera roll is reached
  through the share sheet, which means the Web Share API.

  *Gem* is now a button that fetches the bytes and hands them to `navigator.share` as a `File`. On
  iOS 16.4+ that opens the share sheet with *Gem billede*. Both baselines support sharing files
  (iOS 16.4+, Chrome 111+); where they do not — desktop — it falls back to an **object-URL** anchor,
  which is the correct behaviour there anyway and still not a navigation the service worker can
  answer.

  Three details worth keeping:

  - **It refuses to save a non-media response.** If the fetch ever returns something that is not an
    `image/` or `video/`, saving it under a `.jpg` name would put the app shell in a camera roll
    again. The guard is cheap and names the exact bug.
  - **A cancelled share sheet is not an error.** iOS throws `AbortError` when the member dismisses
    it, and reporting that would accuse them of a mistake they did not make.
  - **Fetching makes this work for a queued glimt too**, whose bytes exist only as a `blob:` URL.
    While here: the viewer was using `glimtMediaUrl(glimt.id, …)` unconditionally, so opening a
    *pending* glimt would have shown a broken image and downloaded the shell — the same bug from the
    other direction. It now prefers `localUrl`, as the strip already did.

  Guarded by `vue/src/navigationFallback.spec.ts` (renamed from `publicPageNotSwallowed.spec.ts`,
  since the invariant is broader than the public page): the denylist covers `/api/` in the config
  **and in the built `sw.js`**, the viewer has no `download` anchor, it goes through
  `navigator.canShare`/`share`, and it checks the response type. **Verified to fail when either bug is
  reintroduced.**

  923 Vue tests, `type-check` and `build` clean.

### Still outstanding

- [ ] **Re-test on the device**: does *Gem* now open the share sheet, and does *Gem billede* put the
      photograph in the camera roll? The mechanism is now the right one, but that is reasoning, not
      evidence — which is exactly what this task exists to refuse.
- [ ] Worth checking at the same time: the icon should now clear the status bar.
