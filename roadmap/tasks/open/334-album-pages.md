# 334 — Album pages

**Status:** open
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `GET /offentligt/album/{slug}` renders one album: title, description, and its photographs in
      curator order with captions where present. OpenAPI annotated.
- [ ] `GET /api/public/albums/{id}/media/{ordinal}` serves album media, mirroring the glimt media
      route's shape and its `?variant=thumb` handling.
- [ ] The frontpage section shows 3–5 covers with titles, linking through.
- [ ] Every media item renders as its own `<img>`; **no carousel, no script, no gestures**.
- [ ] Thumbnail variant by default, `loading="lazy"`, `decoding="async"`, intrinsic `width`/`height`.
- [ ] Alt text on every photograph.
- [ ] A card with no caption does not reserve space for one.
- [ ] Mixed portrait and landscape in one album lays out sanely — the case that catches grid bugs.
- [ ] Works with JavaScript disabled and readable with CSS disabled.
- [ ] An unpublished album is a 404, not an empty page.
- [ ] `noindex, nofollow`.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §7 / §10 (Phase 1). Depends on 333, 332.
