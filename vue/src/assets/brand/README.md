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

## The yellow is an estimate

The source specifies **CMYK 9.6 / 0 / 95.2 / 0 with no embedded output
profile**, so there is no single correct sRGB value — it depends on the press
profile the designer worked in.

The value in use is **`#E6EA08`**, interpolated 10% from the ISO/US-coated
yellow anchor (`#FFF200`) toward the cyan+yellow green anchor (`#00A651`).
A naive per-channel conversion gives `#E6FF0C`, which is wrong: it pins green to
255, an acid yellow-green no yellow ink can produce.

**If a brand guide or Pantone reference turns up, replace it.** It appears in
`nathejk-moon.svg`, `icon.svg`, `../../../public/favicon.svg` and as
`BRAND_YELLOW` in `@/config/brand`. Then re-run the generator.

## Regenerating the raster icons

```sh
vue/scripts/generate-icons.sh
```

Writes `pwa-512`, `pwa-192`, `apple-touch-icon` (180), `maskable-512` and
`badge-96` into `vue/public/`. Those PNGs are **checked in**: they are build
inputs that `vite-plugin-pwa` copies through, not build outputs, so commit them
alongside any SVG change.

## Why there are four framings and not one

Each consumer treats the icon differently, and getting this wrong fails
silently:

- **iOS** ignores SVG for `apple-touch-icon` entirely and falls back to a
  screenshot of the page. Hence the PNG. It also applies its own squircle mask,
  so `icon.svg` is deliberately square and unrounded — pre-rounded art gets
  letterboxed against a white gutter.
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
