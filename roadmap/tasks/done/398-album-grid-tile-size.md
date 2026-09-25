# 398 — The album grid displays a 320px thumbnail in a 288px tile

**Status:** done
**Priority:** high
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

First task out of PRD 023 (§2a.2, §7.1), and the one with the best ratio of benefit to risk: it needs no new
machinery, depends on nothing, and is the change a visitor actually notices.

The public album page's grid was `repeat(auto-fit, minmax(18rem, 1fr))` — one photograph per row on a phone,
two where there was room — on the reasoning that an album is for looking at, so the pictures should get the
space. Right about the goal, wrong about the mechanism:

- **An 18rem column is a 288px tile, and the stored thumbnail is 320px on its longest edge**
  (`glimtThumbEdges`). So the tile was very nearly the whole image, and soft on the 2× display every phone
  has.
- An album of 300 photographs therefore asked the browser to lay out and decode 300 near-full-size images.
  **Slower and blurrier than necessary at the same time** — we were paying to upscale.
- Every cover was its own aspect ratio, so no two tiles in a row agreed on height and the labels underneath
  started at a different place in each column.

At ~10rem the 320px thumbnail is a genuine 2× image and a tile costs about a quarter of the decode. Looking
*at* one photograph is what the viewer in PRD 023 is for; the grid's job is finding it.

**The caption and the credit deliberately stay under the tile for now** (PRD 023 §7.1). They move into the
viewer's info panel in task 403, *with* the viewer that replaces them. The credit line is a published
attribution (task 393) and PRD 011's one documented exception to naming no person, so taking it off the page
before there is somewhere else to read it would be a regression dressed up as a layout change. A two-line
label under a small tile is merely less pretty.

Constraints: server-rendered page (`go-server-rendered-pages`) — no Tailwind, no build step, CSS inline in a
Go **raw string**, so no backtick anywhere in the markup, CSS or comments.

## Acceptance Criteria

- [x] The grid's column is no wider than the stored thumbnail can fill at 2×
- [x] Tiles are a uniform square crop, and a tile holds its space before its bytes arrive
- [x] The caption and the photographer's credit still render on the page
- [x] Intrinsic `width`/`height` survive on the `img` (they are the image's real proportions, and
      `TestAlbumPageRendersItsPhotographs` asserts them)
- [x] A guard fails if the column is widened again, and its message says what that costs

## Progress Log

- 2026-09-25 — Task created from PRD 023, approved the same day.
- 2026-09-25 — `publicsite.go`: `.photos` is now `repeat(auto-fill, minmax(10rem, 1fr))` with
  `align-items: start`; each thumbnail is wrapped in a `<span class="frame">` carrying `aspect-ratio: 1/1`
  and `overflow: hidden`, with `object-fit: cover` on the img. Caption and credit kept, one step smaller to
  suit the tile.
- 2026-09-25 — The crop is on the wrapper rather than the `img` on purpose: on the img the ratio collapses
  while the image is loading, so a half-loaded grid jumps as each photograph lands — which is the thing the
  intrinsic `width`/`height` were added to prevent in the first place.
- 2026-09-25 — `auto-fill`, not `auto-fit`, for the reason task 414 hit on the frontpage: `auto-fit` collapses
  empty tracks, so a three-photograph album would stretch to three enormous tiles.
- 2026-09-25 — Added `TestTheAlbumGridDoesNotUpscaleItsThumbnails`, which parses the column width out of the
  rendered stylesheet and compares it to `glimtThumbEdges[0]`. A test about CSS in a suite that cannot execute
  any, because the number *is* the decision and the way it gets undone is somebody widening the column for a
  page that "looks too dense" — a change that looks cosmetic and is not.
- 2026-09-25 — The guard reads the stylesheet with **CSS comments stripped** (`stripCSSComments`), because the
  comment above the rule quotes `minmax(18rem, 1fr)` to explain what was wrong with it. A needle that matches
  the prose explaining the rule is this repo's recurring source-guard bug — four times before this one.
- 2026-09-25 — Checked the guard can actually fail: temporarily restored 18rem, watched it fail with the right
  message, reverted. A guard nobody has seen fail is a guard nobody knows works.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, and
  `go test ./cmd/api/ -run 'Album|Public|Frontpage|Glimt'` all clean.
