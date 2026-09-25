# 422 — The photo credit takes a row's height from every tile

**Status:** done
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"On the album view page, the photo credit is taking too much space, let it overflow the photo in bottom left
> corner."* — maintainer

The credit was a line under each tile. Two costs, and the second is the one that made the grid look untidy
rather than merely tall:

- every credited tile was a square **plus** a line of text, so the grid was taller than it needed to be at a
  tile size chosen (task 398) to fit as many photographs on a screen as possible;
- and only *some* photographs carry a credit, so a grid had rows of two different heights for no reason a reader
  could see.

It is now a small plate over the photograph's bottom-left corner: dark, translucent, rounded to match the
frame's corner, and **out of the layout entirely**, so every tile is exactly a square again.

### Two details that are decisions rather than styling

- **It is not truncated.** A long name wraps onto a second line and covers a little more of the picture. That is
  the right way round for an attribution: the whole point of the field is that somebody is *named* (task 393,
  PRD 011's one documented exception to naming no person), and "Foto: Vibeke K…" would be a worse outcome than a
  slightly obscured corner.
- **It is transparent to the pointer**, and that is load-bearing rather than tidy. The `figcaption` is a sibling
  of the link, not inside it, so a click landing on the credit would otherwise do nothing at all — a dead corner
  on every credited photograph. Transparent, the click passes through to the tile, which is what the visitor was
  aiming at.

The credit stays on the page rather than moving into the viewer's info panel, for the reason task 403 recorded:
an attribution that only renders once a script has run is an attribution we stop making for everybody whose
script did not run.

## Acceptance Criteria

- [x] Every tile in the grid is the same height, credited or not
- [x] The credit is readable over the photograph, in the bottom-left corner
- [x] A click on the credit still opens the photograph
- [x] A long credit is not truncated
- [x] `TestAlbumPageShowsThePhotographersCredit` still passes unmodified

## Progress Log

- 2026-09-25 — Reported and fixed in one pass. The figure gains `position: relative`; the figcaption is absolute
  at `left: 0; bottom: 0` with a translucent plate; `.photos .credit` now inherits colour and size from the plate
  rather than setting its own, because two places deciding the same thing is how one of them gets forgotten.
- 2026-09-25 — Guarded inside `TestTheCreditStaysOnThePageWhileTheCaptionMovesToTheViewer`, which already owned
  the question of where the credit lives: the plate must be positioned and pointer-transparent, and must **not**
  carry `text-overflow` or `nowrap`.
- 2026-09-25 — Walked into the Go raw-string backtick trap again, in a comment describing `pointer-events`. Sixth
  time in this repo and the second by my hand; the compiler points at a line of CSS, which is why it is worth
  recognising rather than debugging.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean.
