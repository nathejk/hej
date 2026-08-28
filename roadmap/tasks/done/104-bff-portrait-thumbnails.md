# 104 — BFF: server-side thumbnail generation at upload

**Status:** done
**Priority:** low
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

Depends on task 103. (Task 102's consent blocker was cleared 2026-08-28.)

PRD 007 syncs portrait thumbnails to devices for offline identification, so the
thumbnail must be produced once at upload rather than per request: EXIF-correct
orientation, a fixed size, stored alongside the full image.

## Acceptance Criteria

- [x] A fixed-size thumbnail is generated and stored on every portrait write.
- [x] Orientation is correct for photos taken in every device orientation.
- [x] Generation failure fails the upload rather than storing a portrait with no
      thumbnail.
- [x] Test with a rotated fixture image.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Picked up. Found the criteria assumed something task 105 had left
  open: "EXIF-correct orientation" is impossible with Go's `image/jpeg` alone, which
  ignores EXIF entirely. Task 105 had pushed rotation onto the client and flagged the
  `<input capture>` fallback as a risk for task 108. **Doing it properly here closes
  that risk instead of carrying it to a device test.**
- 2026-08-28 — New package `internal/imaging`, holding two things the repo had no
  dependency for: an EXIF-orientation reader and an area-averaging downscaler.
  Decided **against** adding `golang.org/x/image` plus an EXIF library: both operations
  are narrow, and both are testable against *constructed* inputs — an EXIF header can
  be built byte by byte, which a binary fixture cannot be varied into. Noted in the
  package doc where a real library would earn its place (colour management, HEIC).
- 2026-08-28 — All eight EXIF orientations are applied through **one coordinate
  mapping** rather than eight loops. The bespoke version is exactly where 6 and 8 get
  swapped, which is the classic "everyone's photos are upside down" bug; a test now
  walks all eight and checks whether the axes swapped.
- 2026-08-28 — Orientation is applied **once, before both sizes are produced**, so the
  thumbnail cannot disagree with the full image about which way up the face is. A test
  asserts they agree, because applying it twice on one path is the plausible mistake.
- 2026-08-28 — Replaced task 105's nearest-neighbour scaler with **area averaging**.
  At the thumbnail's ratio nearest-neighbour discards 15 of every 16 pixels and eats
  precisely the detail — eyes, hairline — that makes a face recognisable. It is a dozen
  lines and no dependency. Pinned by a test on alternating black/white columns:
  averaging must yield mid-grey, nearest-neighbour would yield pure black or white.
- 2026-08-28 — Thumbnail is **256px longest edge, aspect preserved, not cropped
  square**. Sized against its consumer (PRD 007 caches many faces offline): ~15–25 KB
  each, so hundreds is megabytes not hundreds of megabytes. Cropping is left to the
  client's circular avatar — a server-side square crop would permanently discard the
  sides of a face that was framed slightly off.
- 2026-08-28 — Served as `GET /api/me/photo?size=thumb`. A query parameter, not a
  second route: it selects a representation of the same resource, and PRD 007 will need
  the same choice for other people's portraits — one convention beats two. An
  unrecognised value, or a portrait with no thumbnail, falls back to the full image; a
  client asking for something small would rather have something large than a 404.
- 2026-08-28 — `thumbRef` on the event and `portraitThumbRef` on the projection, both
  optional. A **malformed** thumbnail ref is dropped while the portrait is still
  written: losing someone's photo over a secondary artefact would be the wrong trade.
  A *failed generation*, by contrast, fails the upload — per this task's criterion, so
  "portrait without thumbnail" never becomes a state PRD 007 has to handle forever.
- 2026-08-28 — Fixed a fixture that had now broken twice on additive schema changes:
  the person querier's sqlmock rows are built **by column name** instead of as a
  positional literal. The positional version failed with "expected 21 destination
  arguments, not 22", which names no column; a rename still fails loudly in
  `scanPerson`, where it should.
- 2026-08-28 — Verified live: re-uploaded a PNG, then fetched both sizes —
  `512x512` (10,409 B) and `256x256` (4,489 B), both decodable JPEG. Incidentally
  demonstrated content addressing again: the full image's hash was **identical** to the
  previous upload of the same file.
- 2026-08-28 — ✅ All criteria met. `gofmt -l`, `go test ./...`, `go vet ./...`,
  `staticcheck ./...` green. Rotation is proven by constructed-EXIF unit tests, not by
  a real phone — a real camera file is still worth one check in task 108, but the
  server no longer *depends* on the client getting it right. Moving to done.
