# 110 — Portrait renditions: per-thumbnail size metadata, and more than one thumbnail

**Status:** done
**Priority:** medium
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

Follow-up to task 104, from a maintainer review of the actual event on the stream.

Two problems with the shape 104 shipped:

1. **The thumbnail had no size metadata.** `bytes`, `width` and `height` on
   `PortraitCaptured` described the *full* image only, so a consumer knew the
   thumbnail's hash and nothing else. That defeats the thumbnail's main consumer:
   PRD 007 has to answer "how much storage does caching 800 faces cost?" *before*
   downloading them, and could not.
2. **There will be more than one thumbnail.** An identification grid wants a
   different size from an avatar. A single `thumbRef` would mean changing an event
   shape that is already on an append-only log.

So: a **list of renditions**, each carrying its own name, ref, content type, byte
count and dimensions.

## Acceptance Criteria

- [x] `PortraitCaptured.thumbs` is a list; each entry has `name`, `ref`,
      `contentType`, `bytes`, `width`, `height`.
- [x] The old single `thumbRef` is still *read*, so events already on the log keep
      their thumbnail through a replay.
- [x] The generated sizes are configuration (`thumbnailEdges`), so adding one needs
      no shape change anywhere.
- [x] The projection stores the whole set, and the hot "serve a thumbnail" read
      still needs no JSON parsing.
- [x] `GET /api/me/photo` can serve a named rendition; an absent one falls back to
      the full image.
- [x] The purge deletes **every** rendition, derived from one function so a new
      size cannot be missed.
- [x] Tests, incl. multiple sizes end to end; `go test`/`vet`/`staticcheck` green.

## Progress Log

- 2026-08-28 — Task created after the maintainer read the real message off
  `NATHEJK` and asked for per-thumbnail `bytes`/`width`/`height`, noting there will
  probably be more than one thumbnail.
- 2026-08-28 — Designed for N from the start rather than adding three fields for the
  one thumbnail that exists today. Three fields would have been the cheaper change
  now and the expensive one in a month: this event is on an **append-only log**, so
  every shape it has ever had must stay interpretable forever. Better to pay once.
- 2026-08-28 — `imaging.Prepare` now takes a list of edges and returns
  `Portrait{Full Rendition, Thumbs []Rendition}`. Renditions are named from their
  size (`thumb256`) rather than labelled (`small`): a label needs a table somewhere
  to say what it means, and that table is what drifts from the pixels.
- 2026-08-28 — **The deprecated `thumbRef` is kept for reading.** Only two such
  events exist, both in dev, so deleting the field would have been tempting — but
  the habit is what matters: an event shape that has been published cannot be
  un-published, and "old messages silently lose their thumbnail on replay" is a bug
  that would be invisible. The consumer promotes it into the list so the rest of the
  code has exactly one shape to understand. Documented when it can go: once no
  captured event predating the list remains on the stream.
- 2026-08-28 — Storage: the set goes in a `portraitThumbs` **JSON column**, with the
  smallest rendition also denormalized into `portraitThumbRef` so the common
  "serve this person's thumbnail" read needs no parsing — the same trade this
  projection already makes with `teamName`/`sectionName`. JSON rather than a side
  table because the set is small, always read with its person, and written by one
  event; the doc names the condition for normalizing later (a query *across*
  renditions, e.g. total thumbnail bytes for a year).
- 2026-08-28 — Default thumbnail is the **smallest**, not the first: it is served
  wherever "a thumbnail" is wanted, so the cheapest is the right default. A
  rendition with unknown dimensions (the deprecated shape) sorts last rather than
  winning by comparing zero.
- 2026-08-28 — `Person.PortraitRefs()` is now the single definition of "every object
  this portrait occupies", used by both the retention query and the purge. Without
  it, adding a size would eventually leave a recognisable face on disk after the
  record said the portrait was deleted — so `PortraitPurged` now carries `refs` (a
  list) rather than one `ref`.
- 2026-08-28 — Serving: `size=thumb` gives the default, `size=thumb256` or
  `size=256` names one, unknown or absent falls back to the full image. Case- and
  prefix-insensitive, because a client writing `Thumb256` is not making a mistake
  worth answering with the wrong image.
- 2026-08-28 — The by-name sqlmock fixture from task 104 paid off immediately: the
  new column cost **one line** in each of two test files instead of another round of
  "expected 22 destination arguments, not 23".
- 2026-08-28 — Tested with three sizes (512/256/96) that each rendition carries its
  own dimensions, that they are distinct files, and that a smaller edge really does
  produce a smaller file — otherwise the size list buys nothing.
- 2026-08-28 — **Verified live.** The message on `NATHEJK` now reads:
  `"thumbs":[{"name":"thumb256","ref":"892203d6…","contentType":"image/jpeg","bytes":4489,"width":256,"height":256}]`
  — still no pixels, ~700 bytes. The row stores the same JSON, and the endpoint
  serves `''`→512×512/10,409 B, `?size=thumb`/`thumb256`/`256`→256×256/4,489 B, and
  `?size=thumb96` (not generated) → falls back to the full image.
- 2026-08-28 — Left `thumbnailEdges = []int{256}` unchanged: adding a second size is
  now a one-line configuration change, but *which* sizes PRD 007's grid wants is a
  product question, not mine to invent. Noted that changing the list does not
  rewrite existing portraits — renditions are produced at upload, so a backfill
  would be its own task.
- 2026-08-28 — ✅ All criteria met. `gofmt -l`, `go test ./...`, `go vet ./...`,
  `staticcheck ./...` green.
