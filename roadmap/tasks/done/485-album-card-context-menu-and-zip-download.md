# 485 — Album cards get a context menu, and an album downloads as a zip

**Status:** done
**Priority:** medium
**Created:** 2026-10-01
**Picked up by:** agent session
**Started:** 2026-10-01
**Completed:** 2026-10-01

## Description

Recorded **after the fact**: this work was done at the maintainer's direct request before PRD 027 existed, and the
board should account for it rather than leave it as an unexplained diff. Its ID is higher than 476–484 even though the
work came first; IDs are identifiers, not an order.

Two requests, one session:

1. *"In the edit album list view, the cards are getting a little messy, more space for the cover image and then all
   buttons should be put in a context menu opened by a cog wheel icon top right corner."*
2. *"One additional action: download all images in album as zip-file, sort order same as album, filename:
   `<album name>-<yyyymmdd>.zip`"*

Then, on review: *"The album list should exclude deleted albums."*

**The card.** The cover becomes the card — full column width at 4:3 — and every action moves into a menu behind a cog
pinned to the top-right corner. The card was a 44px thumbnail beside a wrapping row of up to four buttons, which at ten
albums read as a wall of text with a stamp next to it; a curator recognises an album by *looking* at it, because the
title is "Lørdag morgen" and that is true of two hundred photographs.

**The download.** The whole album as one zip, in the album's own order, at a chosen size. The order is the point and it
is carried **twice**: entries are written in sequence *and* each name is prefixed with it, because a zip reader is free
to list entries alphabetically and most do.

**Deleted albums leave the list.** They answer none of the questions the landing page is for, they offer no action but
«Redigér», and a year's deletions accumulate forever — so the answer got quieter every time somebody tidied up. The
filter is per-caller rather than in `CuratorQueries.All`, which other callers need whole.

## Constraints

Server-rendered admin tool (`go/cmd/api/adminui/`): Go `html/template` with vendored Pico + htmx + Alpine, no
Tailwind, no build step, no npm or CDN dependency. Icons are inline SVG copied from Lucide.

## Acceptance Criteria

- [x] The cover is the full width of the card; the actions are in one menu behind a cog
- [x] The menu closes on Escape, on an outside click, and cannot be reopened by its own trigger
- [x] `GET /api/admin/albums/{albumId}/zip` streams the album in order, uncompressed, named `<slug>-<yyyymmdd>.zip`
- [x] Entry names carry the album's order as a numeric prefix
- [x] Three sizes offered, each naming its longest edge
- [x] Deleted albums are not in the list, and nothing addressed to them is either
- [x] OpenAPI annotations on the new endpoint
- [x] Full gate clean: `gofmt`, `go vet`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created retroactively, to record work already done. See the commit for the code.
- 2026-10-01 — `@click.outside` sits on the cog **wrapper**, not on the menu. On the menu, the cog itself counts as
  "outside": the handler closes on the way down and the cog's own toggle reopens on the way up, giving a cog that
  cannot be pressed twice.
- 2026-10-01 — Cover switched from `variant=thumb` to `variant=medium`. A 320px thumbnail stretched across a 15rem
  card is visibly soft on a laptop, and the 800px rendition already exists for exactly this (task 433).
- 2026-10-01 — Zip entries are `zip.Store`, not deflated: JPEG is already entropy-coded, so deflating costs CPU
  proportional to the whole album and saves a percent. The archive exists to make the selection one file in one order,
  not to make it smaller.
- 2026-10-01 — Streamed, not buffered. The consequence is stated in the file header rather than designed away: the
  status line goes out before the first object is opened, so a read that fails halfway cannot become a 500. It is
  logged and the writer is **deliberately not closed**, because an unterminated archive is one a zip reader rejects —
  where a tidily closed one short of its photographs is a file somebody would pass on without noticing.
- 2026-10-01 — Filename uses the **slug**, not the title: `Content-Disposition` is the one place in HTTP where a
  non-ASCII byte needs a second, differently escaped parameter, and the browser's fallback when it disagrees is
  mojibake. The date is when the copy was taken, in the event's timezone, which is what distinguishes two exports of
  an album somebody has since extended.
- 2026-10-01 — **The size labels caused the next piece of work.** The largest was initially offered as "Original",
  which it was not: the upload path normalised everything to 1600px and discarded the photographer's file. Renamed to
  `XLarge (1600px)` with a test forbidding the word, and that gap became **PRD 027**.
- 2026-10-01 — ✅ `gofmt`, `go vet`, full `go test ./...` green.
