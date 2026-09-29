# 463 — The diploma thumbnail had a hole where the photograph goes

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Raised by the maintainer while deciding what a shared patrol page should preview (PRD 026):

> regarding diploma can we construct a start photo with no members that we can embed in the diploma thumbnails,
> then it would look more real

The diploma's artwork leaves a band in the middle for the patrol's photograph, and `diploma.Thumbnail` was a scaled
copy of the bare artwork — so the preview on a patrol's public page had an empty gap where the real document has a
picture. It did not look like a diploma, which matters more now that PRD 026 makes this thumbnail the image Facebook
shows when a parent shares the page.

## What shipped

The photograph's box is filled with **the start line's backdrop, with nobody in front of it**, supplied by the
maintainer during the work and embedded as `internal/diploma/assets/startphoto-standin.jpg`.

I had started drawing one — a night gradient, a lit patch of ground, a vignette, deterministic grain — and deleted
it when the photograph arrived. Worth recording why, because "just draw it" is the obvious shortcut: **a drawn night
scene reads as a drawn night scene.** The first render looked like a placeholder box, and the second, after tuning
the palette and the lights, looked like a placeholder box with better lighting. What makes a preview look real is
that it is real.

### Three things this had to get right

**The geometry is shared with the PDF.** `photoXMM/YMM/WMM/HMM` were local constants inside `drawPhoto` and are now
package-level, with `photoBoxIn()` deriving the pixel rectangle from them. Two copies of those numbers would drift
the first time the artwork moved, and the symptom — a thumbnail whose picture sits where the certificate's does not —
is visible only to somebody holding the two side by side, which is exactly what a family does.
`TestTheThumbnailsPhotoBoxMatchesThePDFs` pins the mapping as fractions of the page.

**It must never reach the PDF**, and the maintainer confirmed the reasoning:

> no as you argued yourself, a patrulje with no photo should not have an empty photo in their pdf, the pdf's stays
> as they are today.

A patrol with no photograph either was not photographed or **has a refusal recorded in hq's Fototilladelse**, and
filling their certificate's empty box with the backdrop would be inventing the photograph they declined. The
thumbnail is illustrative — one image for every patrol, saying "this is a diploma"; the document is not.
`drawPhoto` says so at the line somebody would change, and `TestTheStandInNeverReachesThePDF` looks for the asset's
own bytes in a photographless render.

**No people in it**, which is not a stylistic choice: a stand-in with figures would be a photograph of children that
no consent gate ever saw (PRD 011 §0b.2), on an unauthenticated route.

### A bug found by testing, not by looking

The composite was built at full resolution and scaled afterwards, reasoning that the box's edges would land on the
artwork's own pixel grid. **`imaging.Fit` never enlarges** — it returns the image unchanged when it already fits — so
a 720-pixel photograph placed in an 1180-pixel box was drawn at native size with parchment around it. The rendered
thumbnail looked *almost* right, which is the worst kind of wrong; the test that samples inside the box caught it.

The fix is also the better order: scale the page first, then place the photograph into the ~200-pixel box, which
downscales it once instead of upscaling it and throwing the pixels away. And `standInFor` now **fails loudly** when
a scaled asset cannot fill the box, so a future replacement that is too small is a wrong binary rather than a thin
parchment border nobody would attribute to the asset.

## Acceptance Criteria

- [x] The thumbnail shows a photograph where the diploma has one
- [x] It pictures nobody
- [x] Its box is the PDF's box, from one set of numbers
- [x] The stand-in never appears in a PDF
- [x] An asset that cannot fill the box is an error, not a border
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Drew a stand-in, rendered it, looked at it, and threw it away twice: the first had the backdrop's
  yellow moon in it, which sits a few centimetres below the poster's own large moon and read as a mistake. Then the
  maintainer supplied a real photograph, which settled it.
- 2026-09-28 — Found the `imaging.Fit` no-upscale behaviour the hard way. Both the fix and the guard are recorded at
  the code.
