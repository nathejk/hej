# iOS launch images

Generated — do not hand-edit. Run `vue/scripts/generate-splash.sh`, which also
writes the `<link>` block for `vue/index.html` to
`../../src/assets/brand/startup-links.html`.

40 files: 20 unique device configurations × 2 orientations, ~1.1 MB total.

## Why these exist

iOS does **not** use the manifest's `background_color` for the launch screen —
that only affects Android. With no `apple-touch-startup-image` iOS shows plain
white. It then matches on the device's **exact** dimensions and falls back to
white if nothing matches, which is why this set is exhaustive rather than a
handful of representative sizes.

Filenames are the image's real pixel dimensions, `splash-<width>x<height>.png`.
The pixel size is the CSS size × the device pixel ratio.

> A file whose real dimensions don't match its declared size is **silently
> ignored** by iOS, and the result looks identical to the bug this set fixes. The
> generator verifies all 40 with `sips` after rendering.

## iPhone

One row per unique (CSS size, DPR) triple, since several models share one.

| CSS size | DPR | Portrait | Landscape | Models |
|---|---|---|---|---|
| 375×667 | 2 | `splash-750x1334.png` | `splash-1334x750.png` | iPhone SE (2nd/3rd gen), 8, 7, 6s |
| 414×736 | 3 | `splash-1242x2208.png` | `splash-2208x1242.png` | iPhone 8 Plus, 7 Plus, 6s Plus |
| 375×812 | 3 | `splash-1125x2436.png` | `splash-2436x1125.png` | iPhone X, XS, 11 Pro, 12 mini, 13 mini |
| 414×896 | 2 | `splash-828x1792.png` | `splash-1792x828.png` | iPhone XR, 11 |
| 414×896 | 3 | `splash-1242x2688.png` | `splash-2688x1242.png` | iPhone XS Max, 11 Pro Max |
| 390×844 | 3 | `splash-1170x2532.png` | `splash-2532x1170.png` | iPhone 12, 12 Pro, 13, 13 Pro, 14 |
| 428×926 | 3 | `splash-1284x2778.png` | `splash-2778x1284.png` | iPhone 12 Pro Max, 13 Pro Max, 14 Plus |
| 393×852 | 3 | `splash-1179x2556.png` | `splash-2556x1179.png` | iPhone 14 Pro, 15, 15 Pro, 16, 16e |
| 430×932 | 3 | `splash-1290x2796.png` | `splash-2796x1290.png` | iPhone 14 Pro Max, 15 Plus, 15 Pro Max, 16 Plus |
| 402×874 | 3 | `splash-1206x2622.png` | `splash-2622x1206.png` | iPhone 16 Pro |
| 440×956 | 3 | `splash-1320x2868.png` | `splash-2868x1320.png` | iPhone 16 Pro Max |

## iPad

All at DPR 2.

| CSS size | Portrait | Landscape | Models |
|---|---|---|---|
| 744×1133 | `splash-1488x2266.png` | `splash-2266x1488.png` | iPad mini (6th/7th gen) |
| 768×1024 | `splash-1536x2048.png` | `splash-2048x1536.png` | iPad 9.7", iPad mini 4/5, iPad Air 1/2 |
| 810×1080 | `splash-1620x2160.png` | `splash-2160x1620.png` | iPad 10.2" (7th–9th gen) |
| 820×1180 | `splash-1640x2360.png` | `splash-2360x1640.png` | iPad (10th gen), iPad Air 4/5, Air 11" M2 |
| 834×1112 | `splash-1668x2224.png` | `splash-2224x1668.png` | iPad Pro 10.5", iPad Air 3 |
| 834×1194 | `splash-1668x2388.png` | `splash-2388x1668.png` | iPad Pro 11" (1st–4th gen) |
| 834×1210 | `splash-1668x2420.png` | `splash-2420x1668.png` | iPad Pro 11" M4 |
| 1024×1366 | `splash-2048x2732.png` | `splash-2732x2048.png` | iPad Pro 12.9" (all gens) |
| 1032×1376 | `splash-2064x2752.png` | `splash-2752x2064.png` | iPad Pro 13" M4 |

## Known gap

Models released after this list was written — **iPhone 17 and later** — will show
a white launch screen until their dimensions are added to `DEVICES` in
`vue/scripts/generate-splash.sh`. **This table is maintained by hand and must be
updated in the same commit**, or the two will drift.

## Gotchas

- **`device-width`/`device-height` in the media queries describe the screen and
  do not swap with orientation.** Only the `orientation` term and the image's
  pixel dimensions do. A portrait and landscape pair therefore share identical
  `device-*` values.
- **iOS caches the manifest, icons and startup images at install time.** After
  changing any of them the home-screen app must be deleted and re-added;
  reloading does nothing.
- These are deliberately **not** in the Workbox precache — the OS reads them at
  launch, the app never does at runtime. Verify with
  `grep -c splash- vue/dist/sw.js` after a build; it should print `0`.
