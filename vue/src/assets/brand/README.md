# Brand assets

Vector masters for the Nathejk marks, plus the raster app icons derived from
them. **Edit the SVGs here, never the PNGs in `vue/public/`** — those are
generated.

## Files

| File | Purpose |
|---|---|
| `nathejk-moon.svg` | The moon on its own, tight bbox, no ground. The reusable master — start here for anything new. |
| `icon.svg` | App icon, `any` purpose. Full-bleed square on the dark ground, moon at 70% height. |
| `icon-maskable.svg` | App icon, `maskable` purpose. Moon at 64% so it survives Android's circle crop. |
| `badge.svg` | Notification badge. Bare crescent on transparency. |
| `splash.html` | Render source for the iOS launch images. HTML, not SVG — it has to centre the mark across ~20 aspect ratios. |
| `startup-links.html` | Generated `<link rel="apple-touch-startup-image">` block, to paste into `index.html`. Not an asset; kept out of `public/` so it isn't deployed. |
| `source/Nathejklogo_hvidtekst.eps` | The original artwork, archived. |

`vue/public/favicon.svg` is a fourth framing of the same path, kept in `public/`
because it is served directly.

## Where the moon came from

`source/Nathejklogo_hvidtekst.eps` is the official logo as supplied (Adobe
Illustrator 16, 2017-03-21, for Rasmus Udsholt), archived here because it was
otherwise a single copy on one laptop and every asset below derives from it.

It contains exactly two objects: this moon, and the word `NATHEJK` set in
Impact as an embedded Type 1 subset. Usefully, the artwork is stored as plain
PostScript rather than only inside Illustrator's private data blob, so
`nathejk-moon.svg` is a **verbatim transcription of the original Bézier control
points** — not a trace of a raster preview, and not lossy. The EPS page
transform (`1 -1 scale 0 -199.093 translate`) already maps those coordinates
into a top-left-origin space, i.e. SVG's convention, so they transfer unchanged.

The viewBox `0 0 109.965 150.907` is the path's exact tight bounding box, with
the Bézier extremes solved analytically rather than taken from the control-point
hull. So the mark bleeds to all four edges — **add your own margin at the point
of use**. The height also happens to match the wordmark's baseline in the
source, so the moon is optically aligned to `NATHEJK` by construction.

The wordmark itself has **not** been vectorised. If it is ever needed as an
asset, convert the text to outlines in Illustrator first; do not re-set it with
a font stack, because the glyph widths in the original are Impact's exact
metrics.

## The yellow: `#E6EA08` (confirmed)

**Confirmed 2026-08-27. Do not change it without the same confirmation.**

The derivation is recorded here because the source cannot settle the question on
its own: the EPS specifies **CMYK 9.6 / 0 / 95.2 / 0 with no embedded output
profile**, so it has no single correct sRGB rendering — that depends on the press
profile the designer worked in.

`#E6EA08` was arrived at by interpolating 10% from the ISO/US-coated yellow
anchor (`#FFF200`) toward the cyan+yellow green anchor (`#00A651`), then
confirmed against the artwork. For the record, a naive per-channel conversion
gives `#E6FF0C`, which is wrong: it pins green to 255, an acid yellow-green no
yellow ink can produce — don't "correct" the value to that.

If it ever does need to change, it appears in `nathejk-moon.svg`, `icon.svg`,
`../../../public/favicon.svg` and as `BRAND_YELLOW` in `@/config/brand` — then
re-run the generator.

## Regenerating the raster icons

```sh
vue/scripts/generate-icons.sh
```

Writes `pwa-512`, `pwa-192`, `apple-touch-icon` (180), `maskable-512` and
`badge-96` into `vue/public/`. Those PNGs are **checked in**: they are build
inputs that `vite-plugin-pwa` copies through, not build outputs, so commit them
alongside any SVG change.

## Regenerating the iOS launch images

```sh
vue/scripts/generate-splash.sh
```

Writes 40 PNGs (20 device configurations x 2 orientations, ~1.1 MB total) into
`vue/public/splash/`, plus `startup-links.html` — paste that between the
`START`/`END apple-touch-startup-image` markers in `vue/index.html`.

**iOS does not use the manifest's `background_color` for the launch screen.** With
no startup image it shows plain white; the manifest value only affects Android.
iOS then matches on the device's **exact** dimensions and falls back to white if
nothing matches, which is why the set is exhaustive rather than one or two sizes.
Models newer than the device list in the script (iPhone 17 and later) will show
white until their dimensions are added.

These are deliberately **not** in the Workbox precache — 1.1 MB of images the OS
reads at launch, never the app at runtime. Verify with
`grep -c splash- vue/dist/sw.js` after a build; it should print `0`.

> **iOS caches the manifest, icons and startup images at install time.** After
> changing any of them you must delete the home-screen app and re-add it —
> reloading does nothing.

## Why there are four framings and not one

Each consumer treats the icon differently, and getting this wrong fails
silently:

- **iOS** ignores SVG for `apple-touch-icon` entirely and falls back to a
  screenshot of the page. Hence the PNG. It also applies its own squircle mask,
  so `icon.svg` is deliberately square and unrounded — pre-rounded art gets
  letterboxed against a white gutter. And it ignores `background_color` for the
  launch screen, hence the 40 startup images.
- **Android** crops `maskable` icons to a launcher-chosen shape, guaranteeing
  only the centred circle of 80% width. The moon's extremities are its two thin
  horns, exactly what a circle crop shaves off, so the maskable variant is
  framed smaller. Its ground must bleed to all four edges — never pad a maskable
  icon with transparency.
- **Notification badges** are rendered from the **alpha channel alone**, tinted
  by the system. Passing an app icon yields a solid square, because its dark
  ground is opaque edge to edge. Hence `badge.svg`.
- **Favicons** are drawn as authored at 16px, so that one carries its own
  rounded corners and is framed tightest.
