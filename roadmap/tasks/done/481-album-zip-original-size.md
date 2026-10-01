# 481 — size=original in the album zip

**Status:** done
**Priority:** medium
**Created:** 2026-10-01
**Picked up by:** agent session (PRD 027 rollout)
**Started:** 2026-10-01
**Completed:** 2026-10-01

**PRD:** 027 (R5)
**Depends on:** 479

## Description

The album zip is how a curator gets a set of photographs out of the archive, and it is the surface that made the gap
visible in the first place: it once offered a size called "Original" that handed over the 1600px display image. The
menu was corrected to `XLarge (1600px)` and a test was added forbidding the word — a guard that documented the gap
rather than closing it. This task closes it, and the word becomes honest.

In `cmd/api/adminalbumzip.go`: a **fourth entry in `adminZipSizes`** reading `originalRef`, with the fallback in
`adminZipRendition` falling back to `xlarge` when a photograph has no original. The fallback is not a nicety — it is
every photograph uploaded before PRD 027 ships, and there is no backfill, so a mixed album is the normal case for the
life of the archive. Task 482 is what makes the substitution visible in the UI; this task must make it correct in the
bytes.

**The existing three sizes are unchanged and stay metadata-stripped.** They are renditions; PRD 027 R12 keeps them
exactly as they are. Only the new entry streams a file with EXIF in it.

Per `.rules`, the **OpenAPI annotations must be updated in the same change** — `size` on
`GET /api/admin/albums/{albumId}/zip` gains `original`, and the fallback behaviour belongs in the description, since
a caller cannot otherwise know the archive may contain a smaller file than asked for.

**Note on the existing test.** `TestTheAlbumListOffersTheZipDownload` currently asserts that the word "Original"
does **not** appear in the menu. That assertion is **inverted by this task, not deleted** — it was recording a
deliberate absence, and the same test should now record the deliberate presence. Deleting it would throw away the
only guard that the label means what it says.

## Acceptance Criteria

- [x] `adminZipSizes` has a fourth entry reading `originalRef`
- [x] `adminZipRendition` falls back to `xlarge` for a photograph with no original
- [x] A test zips an album with a mix of photographs with and without originals and asserts which bytes each entry got
- [x] The three existing sizes are byte-for-byte unchanged and still metadata-stripped
- [x] `TestTheAlbumListOffersTheZipDownload` is inverted to assert "Original" **is** offered, and still exists
- [x] OpenAPI annotations for `GET /api/admin/albums/{albumId}/zip` updated in the same change, including the
      fallback
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created from PRD 027.
- 2026-10-01 — Picked up and done. Fourth entry in `adminZipSizes`, the fallback in `adminZipRendition`, the menu link,
  annotations updated.
- 2026-10-01 — `original` carries a `bool` rather than sharing `edge: 0` with `xlarge`. Both do mean "send what is
  stored", but they read different columns and one of them may be absent — collapsing them into one entry would make
  "which column" implicit in a field called `edge`.
- 2026-10-01 — The original branch is answered **first and without any derive path at all**. Not an optimisation: an
  original that had been through `imaging` would not be an original, and a re-encode would strip the metadata — making
  the download silently not an original while still being called one, which is the exact confusion the `XLarge` rename
  exists to end.
- 2026-10-01 — Placed **last** in the menu, below the three scales, with a rule above it. It is the heaviest and most
  specialised download: a curator sending a photograph to the local paper wants a scale, and reaching past them by
  accident is a multi-gigabyte zip. A rule rather than a colour, because this is a grouping and not a warning — the
  destructive styling in that menu belongs to «Slet album» alone.
- 2026-10-01 — **Inverted an assertion rather than deleting it.** `TestTheAlbumListOffersTheZipDownload` previously
  required the word "Original" to be *absent*, because none was stored. That guard was placed in anticipation of this
  task, so the test now records the inversion and why the name is earned, instead of quietly losing the history.
- 2026-10-01 — ✅ New tests: the original is streamed byte-identically **and still carries GPS** through both the zip
  and the media route (one test, because those are the only two ways an original leaves this service and they share the
  as-stored rule); the fallback works on both; and `TestTheMenuAndTheEndpointAgreeOnTheSizes` checks the two lists in
  two languages against each other in both directions — a link to a size the endpoint rejects is a 400 a curator meets
  after choosing, and a size served but never offered is dead code.
- 2026-10-01 — ✅ `gofmt`, `go build`, full `go test ./...` green.
