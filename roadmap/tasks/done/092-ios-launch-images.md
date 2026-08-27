# 092 — iOS launch images (fix the white boot screen)

**Status:** done
**Priority:** medium
**Created:** 2026-08-27
**Picked up by:** zed-agent
**Started:** 2026-08-27
**Completed:** 2026-08-27

## Description

Task 091 set the manifest `background_color` to `#0f172a` to stop the boot screen
flashing white, and **that fixed Android only**. The user reported the white
splash still present on iPhone.

The premise in 091 was wrong: it assumed iOS 16.4+ would synthesise a launch
screen from the manifest, and on that basis explicitly skipped
`apple-touch-startup-image`. iOS does not use `background_color` for the launch
screen at all — with no startup image it shows plain white, regardless of the
manifest.

Fix: generate the full `apple-touch-startup-image` set. iOS matches on the
device's **exact** dimensions and falls back to white when nothing matches, so
this must be exhaustive rather than one or two representative sizes.

Related: 091 (icons, splash colour, brand assets).

### Not in scope

- **The location re-prompt on every cold launch**, reported alongside this. That
  is WebKit scoping geolocation permission to the browsing session, and it cannot
  be persisted from web code — see the progress log for what *is* fixable and the
  knock-on risk to the track recorder (tasks 081–086). Needs its own task.

## Acceptance Criteria

- [x] Launch images covering the current iPhone and iPad lineup, both
      orientations.
- [x] `<link rel="apple-touch-startup-image">` block wired into `index.html`,
      with the exact media queries iOS requires.
- [x] Repeatable generator script.
- [x] Every generated file's real pixel dimensions verified against its declared
      size (a wrong size is silently ignored by iOS).
- [x] Images kept out of the Workbox precache.
- [x] `npm run build` + `npm run type-check` clean.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-08-27 17:30 — Picked up. Plan: one scalable splash source, rendered at
  every device resolution, plus the generated link block for `index.html`.
- 2026-08-27 17:33 — Decision: splash source is **HTML, not SVG**
  (`src/assets/brand/splash.html`). The mark has to be centred across ~20 aspect
  ratios from 375x667 to 1032x1376; that is one line of flexbox, versus awkward
  per-size viewBox arithmetic in static SVG. Sized in `vmin` so it is
  resolution-independent and keeps constant optical weight on phone and iPad.
- 2026-08-27 17:39 — Rendered a test 1179x2556. Decision: bumped the moon from
  30vmin to 38vmin — at 30 it read as lost on a tall phone canvas.
- 2026-08-27 17:41 — Decision on how to drive Chrome: render at **physical pixel
  dimensions with the default scale factor**, not CSS points with
  `--force-device-scale-factor`. The layout is pure `vmin` so both are
  equivalent, and every phone's CSS width (375–440) would hit the ~500px headless
  window clamp that silently crops instead of scaling (the trap that produced a
  blank badge and an off-centre screenshot in 091). At physical sizes the
  smallest dimension used is 750, comfortably clear of it.
- 2026-08-27 17:44 — Device list: one entry per unique
  (width, height, dpr) triple, since several models share one — iPhone 14 Pro /
  15 / 15 Pro / 16 are all 393x852@3, and a duplicate media query is dead weight.
  20 configurations. Noted in the script that `device-width`/`device-height`
  describe the screen and do **not** swap with orientation; only the
  `orientation` term and the image's pixel dimensions do.
- 2026-08-27 17:52 — Generated 40 images (20 x 2 orientations), 1.1 MB total.
  ✅ Criterion 1.
- 2026-08-27 17:54 — Verified all 40 by comparing `sips` pixel dimensions against
  each filename, rather than spot-checking. This is not optional here: iOS
  silently ignores a mismatched image and shows white, so a wrong size would look
  exactly like the bug being fixed. All 40 correct. ✅ Criterion 4.
- 2026-08-27 17:57 — Inserted the 40 link tags into `index.html` between
  `START`/`END apple-touch-startup-image` markers. ✅ Criterion 2.
- 2026-08-27 17:59 — Moved the generated link block out of `public/` to
  `src/assets/brand/startup-links.html`: it is a copy-paste aid, and anything
  under `public/` is deployed verbatim — a stray `/splash/links.html` on the
  origin would have been sloppy. ✅ Criterion 3.
- 2026-08-27 18:02 — ✅ Verified in `node:20-alpine`: build + `vue-tsc` clean, 40
  images in `dist/splash/`, 42 startup-image references in `dist/index.html` (40
  links + the 2 marker comments). Confirmed **0** splash entries in `dist/sw.js`:
  1.1 MB the OS reads at launch has no business in the runtime precache.
  ✅ Criteria 5, 6.
- 2026-08-27 18:04 — Completed. **Untested on hardware** — needs an iPhone, and
  the home-screen app must be deleted and re-added first, since iOS caches the
  manifest, icons and startup images at install time. That re-add is also what
  will finally surface 091's icon work.
