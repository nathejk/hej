# 334 — Album pages

**Status:** done
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §6 (section 1), §7. The frontpage shows 3–5 album covers with titles; `/offentligt/album/{slug}`
opens one, showing its photographs in the curator's order with optional captions.

**No carousel.** Every photograph renders as its own `<img>`. This is PRD 019's decision (task 323) and
its reasoning transfers exactly: the public surface has to work on a desktop *without a swipe* and with
no script, and the app's Embla-driven strip is a Vue component in a bundle this page does not load. The
rejected alternatives are recorded in task 323 — read it rather than rediscovering them.

**Thumbnails always.** This page is read by a lot of people at once on whatever connection they have,
and a grid of full-size images is the difference between usable and not. `loading="lazy"` and
`decoding="async"` are plain attributes, not script. Emit `width`/`height` so the page does not reflow
as it loads.

Depends on task 333 for storage and ingest, and 332 for the shell.

## Acceptance Criteria

- [x] `GET /offentligt/album/{slug}` renders one album: title, description, and its photographs in
      curator order with captions where present. OpenAPI annotated.
- [x] `GET /api/public/albums/{id}/media/{ordinal}` serves album media, mirroring the glimt media
      route's shape and its `?variant=thumb` handling.
- [x] The frontpage section shows 3–5 covers with titles, linking through.
- [x] Every media item renders as its own `<img>`; **no carousel, no script, no gestures**.
- [x] Thumbnail variant by default, `loading="lazy"`, `decoding="async"`, intrinsic `width`/`height`.
- [x] Alt text on every photograph.
- [x] A card with no caption does not reserve space for one.
- [x] Mixed portrait and landscape in one album lays out sanely — the case that catches grid bugs.
- [x] Works with JavaScript disabled and readable with CSS disabled.
- [x] An unpublished album is a 404, not an empty page.
- [x] `noindex, nofollow`.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §7 / §10 (Phase 1). Depends on 333, 332.
- 2026-09-19 — Picked up. Read task 323's no-carousel reasoning first rather than rederiving it; it
  transfers to albums unchanged, so the page renders one `<img>` per item and nothing else.
- 2026-09-19 — **`publicAlbumItem` deliberately carries no coordinate**, and this is the decision worth
  recording. The projection's `Item` has `Lat`/`Lng`, and passing it straight to the template would have
  been the obvious move — and would have published positions to the open web through a template nobody
  reviewed for that. Positions reach the public only through the map endpoint (task 342), which is the
  one place the decision is made and tested. `TestAlbumPageCarriesNoCoordinates` guards both the album
  page and the frontpage.
- 2026-09-19 — The media route re-checks publication **per request**, because bytes are addressed by
  album id and ordinal: without it an id would be enough to pull a draft album's photograph off an
  unauthenticated route. Same trap the glimt media route documents. Implemented by funnelling the
  id-based lookup through `Published`+`BySlug` rather than adding a `ByID` read — a second read
  returning items would be a second place the publication filter has to be remembered, and this route is
  exactly where forgetting it would matter.
- 2026-09-19 — Reused `streamGlimtMedia` rather than writing a second byte-streamer, so the ETag
  handling, the 304 path and the missing-object-degrades-to-404 rule (PRD 008 §8) cannot diverge between
  two routes doing the same job.
- 2026-09-19 — A missing thumbnail falls back to the full image rather than 404ing, matching the glimt
  grid: a thumbnail is an optimisation, so losing one should cost bandwidth, not the photograph.
  Asserted both ways.
- 2026-09-19 — **503 on the album page when the projection is nil, not 404** — which is the opposite of
  the choice made for patrol pages, and deliberately. The difference is what the answer reveals: a
  closed patrol page must be indistinguishable from a nonexistent one because the difference leaks who
  has finished, while "albums are down" leaks nothing, and a 404 would tell a curator their album had
  vanished.
- 2026-09-19 — The `albumStore` test double implements the publication filter **for real**, including in
  `BySlug`. A stub that ignored `published` would have made the draft-is-unreachable tests pass while
  the behaviour was absent — the same reasoning `stubGlimt.RefsUsedElsewhere` records.
- 2026-09-19 — Verified end to end against the running dev stack with the task 333 fixture, which is
  what it was built for:
  * frontpage lists the two published albums with covers and counts; the draft is absent — no title, no
    slug, no id;
  * the album page renders all four photographs, two with captions and two falling back to
    `alt="Billede fra Lørdag morgen"`, with intrinsic dimensions for both orientations;
  * the media route serves a real 3.5 KB JPEG with `public, max-age=31536000, immutable` and an ETag;
  * `/offentligt/album/kladde` → **404**, and the draft's media → **404**;
  * the stored verdicts cover all three live cases — `inside`, `none`, and `outside` for the Manhattan
    coordinate with its value kept.
- 2026-09-19 — Also ran the `Plottable` query by hand against the fixture data: the draft album's item
  is `inside` but **does not** appear, because the query joins `album.published = 1`. That is the one
  case where an unpublished album could have leaked a position to the map, so it was worth checking
  against real rows rather than trusting the SQL by reading.
- 2026-09-19 — `go vet ./...` and `go test ./...` clean; the OpenAPI guard (widened in task 332) accepted
  both new routes without complaint. Moving to done.
