# 091 — Refine PWA icons and boot screen with the real Nathejk moon

**Status:** done
**Priority:** medium
**Created:** 2026-08-27
**Picked up by:** zed-agent
**Started:** 2026-08-27
**Completed:** 2026-08-27

## Description

Replace the placeholder branding shipped by tasks 009/014 — a dark rounded square
with the letter "H" in `system-ui` — with the real Nathejk moon, and fix the
platform-specific icon bugs that the single-placeholder approach was hiding.

Task 014 deliberately deferred PNG generation ("keeps the skeleton
dependency-light; PNG sizes can be generated later if a target platform needs
them"). Two platforms need them, and both were failing silently:

- **iOS ignores SVG for `apple-touch-icon`.** The home-screen icon was falling
  back to a screenshot of the page, so the app had effectively no icon on the
  primary target platform (`.rules` baseline: iOS 16.4+).
- **Android renders `Notification.badge` from the alpha channel alone.**
  `push-sw.js` passed the app icon for both `icon` and `badge`, so the
  status-bar badge rendered as a solid opaque square.

Also in scope: the boot/launch screen. The manifest had `background_color:
#ffffff` against a `#0f172a` theme, so every cold start flashed white before
cutting to the dark app bar — worst at night, which is when this app is used.

Source artwork: `Nathejklogo_hvidtekst.eps` (Adobe Illustrator 16, 2017-03-21,
for Rasmus Udsholt), supplied by the user from `~/Downloads`.

Related: 009 (branding), 014 (PWA install), 004 (PrimeVue removal — the Nathejk
font/Impact stack this mark aligns to).

### Out of scope / follow-ups

- ~~**The moon's yellow is an unverified estimate.**~~ Resolved 2026-08-27:
  `#E6EA08` confirmed by the design owner. See the final progress log entry.
- **Manifest screenshots cover only `/login`.** Every other route is behind the
  auth guard; capturing maps/rulebook needs a session for a seeded person, which
  needs jetstream up.

## Acceptance Criteria

- [x] Moon extracted from the supplied EPS as a reusable vector master.
- [x] Original EPS archived in-repo (it existed only as a single copy on one
      laptop, and all assets derive from it).
- [x] Separate `any` and `maskable` icon framings — not one file serving both.
- [x] PNG `apple-touch-icon` wired in `index.html`.
- [x] Monochrome notification badge, distinct from the notification icon, wired
      in `push-sw.js`.
- [x] Favicon redrawn with the real mark.
- [x] Placeholder `public/logo.svg` deleted and all references updated.
- [x] Launch-screen background no longer flashes white.
- [x] Manifest `screenshots` (one `narrow`, one `wide`) for the rich install
      prompt.
- [x] Repeatable generation scripts, with the platform gotchas documented.
- [x] `npm run build` + `npm run type-check` clean; emitted manifest and SW
      precache list inspected.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-08-27 15:20 — Picked up. Plan: audit what branding exists, get a vector
  master out of the EPS, then generate the per-platform raster set and rewire the
  manifest.
- 2026-08-27 15:24 — Audited the EPS. It contains exactly two objects: the moon
  path and `NATHEJK` as an embedded Impact Type 1 subset. Crucially the artwork
  is stored as **plain PostScript**, not only inside Illustrator's private data
  blob — so no rasterise-and-trace is needed.
- 2026-08-27 15:26 — Decision: transcode the Bézier control points verbatim
  rather than trace a preview. The page transform
  (`1 -1 scale 0 -199.093 translate`) already maps coordinates into a
  top-left-origin space, i.e. SVG's convention, so they transfer unchanged. Also
  ruled out installing Ghostscript/Inkscape — none present, and unnecessary given
  the above. `qlmanage` hangs on this file.
- 2026-08-27 15:28 — Solved the Bézier extremes analytically (not the
  control-point hull) for a tight viewBox: exactly `0 0 109.965 150.907`. The
  height matches the wordmark baseline in the source, so the mark is optically
  aligned to `NATHEJK` by construction. ✅ Criterion 1: `nathejk-moon.svg`.
- 2026-08-27 15:31 — Blocker: the fill is CMYK 9.6/0/95.2/0 with **no embedded
  output profile**, so there is no single correct sRGB value. Naive per-channel
  conversion gives `#E6FF0C`, which is wrong — it pins green to 255, an acid
  yellow-green no yellow ink can produce.
- 2026-08-27 15:33 — Decision: use `#E6EA08`, interpolated 10% from the
  ISO/US-coated yellow anchor (`#FFF200`) toward the cyan+yellow green anchor
  (`#00A651`). Flagged to the user as an estimate and documented in the brand
  README with every location it appears. **Follow-up: replace if a brand hex
  exists.**
- 2026-08-27 15:36 — Authored three framings (`icon.svg`, `icon-maskable.svg`,
  `badge.svg`) plus a redrawn `favicon.svg`. Maskable is framed at 64% height vs
  70% for `any`: half-diagonal 204.1 against Android's 204.8 safe radius,
  measured conservatively to the bbox corners. The moon's extremities are its two
  thin horns — exactly what a circle crop shaves off — so this mark needs more
  margin than a compact glyph would. ✅ Criteria 3, 6.
- 2026-08-27 15:40 — Gotcha #1: headless Chrome writes its screenshot and then
  hangs on shutdown instead of exiting, and reusing one `--user-data-dir` makes
  the next invocation block on the profile lock. `generate-icons.sh` now
  backgrounds Chrome, polls for a size-stable file, and kills it, with a
  throwaway profile per render.
- 2026-08-27 15:44 — Gotcha #2, and it failed **deceptively**: the 96px badge
  render produced a correctly-sized, entirely blank PNG. Chrome 132+ dropped the
  old headless engine, so headless drives a real browser window and enforces the
  platform minimum window size — a 96px request lays out at ~500px and then
  *crops* the screenshot, yielding the empty top-left corner. Fixed by rendering
  one 1024px master per source and downscaling via `sips`, which also antialiases
  better. Documented in the script.
- 2026-08-27 15:47 — Verified the badge by **decoding its alpha channel**
  (stdlib zlib PNG reader), not by eye: white-on-transparent is invisible in any
  viewer, which is precisely how the blank render survived. Result: 23.9% opaque
  / 72.5% transparent in a clear crescent, with antialiased edges. ✅ Criterion 5.
- 2026-08-27 15:50 — Rewired: manifest now lists three distinct PNGs;
  `index.html` uses `apple-touch-icon.png`; `push-sw.js` splits `icon`
  (`pwa-192.png`) from `badge` (`badge-96.png`); `background_color` and
  `BACKGROUND_COLOR` changed `#ffffff` → `#0f172a`. Deleted `public/logo.svg`.
  ✅ Criteria 4, 7, 8.
- 2026-08-27 15:55 — Archived the source EPS to
  `vue/src/assets/brand/source/` (sha256 `d07a295e…`) and wrote a brand README.
  Decision: did **not** vectorise the `NATHEJK` wordmark — re-setting it from a
  font stack would drift from Impact's exact glyph metrics; outline it in
  Illustrator if ever needed. ✅ Criterion 2.
- 2026-08-27 15:58 — Screenshots: attempted a real session to capture the
  interesting views. There is a dev SMS log-sender that prints the PIN, and
  seeded test numbers in `go/cmd/seed`, but `+4599000010` returns the
  anti-enumeration response with no PIN logged — jetstream is unreachable in the
  local stack, so the person projection is empty. **Blocker: only `/login` is
  publicly reachable.** Captured that at both form factors and shaped the
  manifest to take more entries later.
- 2026-08-27 16:00 — Gotcha #2 resurfaced on the screenshots: the 412px narrow
  capture came out right-sized but visibly off-centre with its right edge cut.
  Confirmed it was the window clamp and not a responsive bug in `LoginView` by
  checking the 1280px capture (correct). `--headless=old` is ignored on Chrome
  152 and `--force-device-scale-factor=2` does not help — the clamp applies to
  the **CSS** width, not the physical one. Narrow capture is therefore 540px
  wide, not a true phone viewport. ✅ Criterion 9.
- 2026-08-27 16:03 — Checked cache impact, given this repo's budget concerns
  (087/PRD 009): the screenshots are **not** precached — only the icons are, 30
  entries unchanged. Install-prompt images are fetched by browser UI, not the
  app, so no `globIgnores` needed.
- 2026-08-27 16:05 — ✅ Verified in `node:20-alpine`: `npm run build` and
  `vue-tsc --noEmit` clean; inspected `dist/manifest.webmanifest` and the `sw.js`
  precache list directly. ✅ Criteria 10, 11. Completed — real mark on every
  surface, two silent platform bugs fixed, generation scripted and documented.
  Two follow-ups remain open (see Description): confirm the yellow, and capture
  authed screenshots once jetstream is available.
- 2026-08-27 16:20 — Follow-up closed: **`#E6EA08` confirmed by the design
  owner.** The value is unchanged, so no icons needed regenerating — but the
  caveat was live in three places telling the next reader to go replace it
  (`nathejk-moon.svg`, the brand README, `BRAND_YELLOW` in `@/config/brand`), so
  all three now record it as confirmed and dated. Kept the derivation on record,
  including the note that the naive per-channel conversion (`#E6FF0C`) is wrong,
  so nobody "corrects" it later. One follow-up still open: authed screenshots.
