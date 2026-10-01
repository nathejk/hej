# 479 — Store the photographer's file unchanged

**Status:** done
**Priority:** high
**Created:** 2026-10-01
**Picked up by:** agent session (PRD 027 rollout)
**Started:** 2026-10-01
**Completed:** 2026-10-01

**PRD:** 027 (R1, R4, R12)
**Depends on:** 477, 478

## Description

Fourth task of **PRD 027** and the irreversible one: after this, the archive holds the photographer's files.

**R4 is scoped to this path and only this path.** Glimt and portrait display images stay `blob.Put` — see PRD 027 R4a:
a participant's photograph from the race is not photographer quality, but it is the most original copy that will ever
exist, so it is an original and stays in the backup scope. The library is the one path where the display image stops
being the only copy, which is the entire reason it may be demoted. Do not generalise this change.

This is the change PRD 027 exists for: the library stops discarding the photographer's pixels. Today
`cmd/api/albummedia.go` re-encodes every upload to 1600px and the handed-in file is gone the moment the request
ends — silently, permanently, with no backfill possible. After this task the uploaded bytes are kept **exactly as
handed in**: full resolution, metadata intact, no re-encode, no strip, no resize, **no condition**.

**In `cmd/api/albummedia.go`:**

- `blobs.Put(ctx, raw)` on the **raw uploaded bytes**, **before** preparing renditions. Before, because the original
  is the thing that cannot be reproduced; if the store is full or failing we should discover it before spending work
  on derivatives. If storing the original fails, **the upload fails** — keeping the photograph and silently dropping
  its original leaves one frame quietly un-recoverable, discovered years later for no visible reason. The portrait
  path already makes exactly this call.
- The 1600px display image moves from `Put` to `PutCache` (R4). It is now *derived* from something the store holds,
  so it belongs in `cache/`. Existing display images stay where they are; `locate` finds either, and `Put`'s
  promotion rule already prevents a sole copy landing in a subtree a backup skips.
- Width, height and content type of the original come from `image.DecodeConfig` on the raw bytes. That is a **header
  read, not a decode** — no second decode is paid for — and the numbers therefore describe **the stored bytes before
  rotation**, exactly as `imaging` already documents for portraits. Say so where the values are produced, because
  "width" that disagrees with what a viewer sees is otherwise read as a bug.
- Carry the original on `albumMediaPrepared`.

**Explicitly do not use `imaging.Prepare(keepOriginal: true)`.** That path strips metadata and declines a same-size
original, and both behaviours are now wrong here: the whole point is that the file keeps its EXIF, and the
"only if it has more pixels" condition rested on a stripped same-size copy carrying no additional information. Once
the original keeps its metadata that premise is gone — capture time, camera, lens and coordinate are information no
rendition has and none can re-derive. So **every upload gets an original**, which also makes `originalRef` mean one
thing rather than "present, unless…". `keepOriginal` stays exactly as it is for portraits, where stripping is still
right; do not touch it.

**In `cmd/api/adminupload.go`:** publish the new nested field, and extend the free-space estimate
(`blob.FreeSpacer`) to account for the original as well as the renditions — otherwise a full volume is discovered
mid-card, halfway through a photographer's SD card.

**The load-bearing test.** `TestStoreAlbumImageReadsTheCoordinateAndStripsIt` must keep passing **unchanged**. It
asserts that the bytes a reader is served carry no GPS, and that is the half of the old "never preserve EXIF" rule
that does not move an inch (R12). The archive gains metadata; no reader does. If that test needed editing to make
this task pass, the change went wrong — stop and re-read PRD 027 §8.

## Acceptance Criteria

- [x] The raw uploaded bytes are stored with `blobs.Put` before renditions are prepared, byte for byte, metadata
      intact
- [x] A failure storing the original fails the upload; no photograph is kept without one
- [x] Every upload gets an original — no size or pixel-count condition anywhere in the path
- [x] The 1600px display image is written with `PutCache`; existing display images still resolve
- [x] Original width, height and content type come from `image.DecodeConfig` on the raw bytes, documented as
      pre-rotation
- [x] `imaging.Prepare`'s `keepOriginal` is unchanged, and is not used for the library original
- [x] `cmd/api/adminupload.go` publishes the nested original and its free-space estimate includes it
- [x] `TestStoreAlbumImageReadsTheCoordinateAndStripsIt` passes **unchanged**
- [x] A test asserts the stored original still carries the EXIF the upload had, including GPS
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created from PRD 027.
- 2026-10-01 — Picked up. Plan: `storeLibraryOriginal` on the raw bytes before `Prepare`, display image to `PutCache`,
  carry it on `albumMediaPrepared`, publish it, widen the disk floor.
- 2026-10-01 — Put the original write in its own function rather than inline. The reasoning it needs is substantial —
  byte-for-byte, no condition, why a failure is fatal, why the dimensions are pre-rotation — and inlining it would
  have buried all of that in the middle of a function that already explains the GPS ordering.
- 2026-10-01 — `image.DecodeConfig` on the raw bytes for width/height/format: a **header** read, so no second decode
  and no need for the decode gate. It also gives a free early rejection of a `.mov` or a `Thumbs.db` off the same
  card, mapped to `errGlimtNotMedia` so the handler's one Danish sentence still covers it — which means nothing
  unreadable reaches the volume that is backed up and never purged.
- 2026-10-01 — Content type derived from what the decoder **recognised**, not from the request's Content-Type, which
  is whatever the client chose to claim. It matters because the original is not re-encoded, so a stored PNG served as
  image/jpeg is a file a curator's tools refuse.
- 2026-10-01 — Disk floor is now `2 * len(raw)`. The old single raw size over-counted, because the only object written
  was smaller than the upload; now the original alone costs exactly the raw size, so the same figure would
  **under**-count — and under-counting a floor is how a volume fills halfway through an SD card. Crude in the safe
  direction, bounded by the 32 MB cap.
- 2026-10-01 — **Two existing tests had to be inverted, and they were right before.**
  `TestAlbumIngestClassifiesItsBytes` and `TestAlbumIngestProducesBothDerivedRenditions` asserted the library's 1600px
  display image is an *original*. That was correct while it was the only copy of those pixels — which is what the blob
  store's classes actually distinguish, "is this the only copy", not "was this re-encoded". Rewrote both to assert the
  new arrangement and, importantly, to state in the test why it changed and that the glimt test above them still
  expects `Put` for a frame that is also a re-encode (R4a). Neither was deleted; a reader needs to find the reversal
  where the old rule was.
- 2026-10-01 — ✅ New tests in `cmd/api/albumoriginal_test.go`, holding **both halves of PRD 027 in one file on
  purpose**: the stored original is byte-identical to the upload and still carries GPS, *and* all three renditions
  carry none. Seeing only one of those is how somebody concludes the old "never preserve EXIF" rule was simply
  dropped. `TestStoreAlbumImageReadsTheCoordinateAndStripsIt` passes **unchanged**, which was the task's own
  stop-condition.
- 2026-10-01 — Also covered: every size gets an original including a 320px upload (no condition); a `Put` failure fails
  the ingest; a non-image stores nothing at all; and a source guard that the library does **not** use
  `imaging.Prepare(keepOriginal: true)` — switching that on would look like the obvious implementation of PRD 027 and
  would silently store censored originals, since that path strips metadata and declines same-size copies. The same
  guard asserts the portrait path still passes it, so this is not read as "keepOriginal is deprecated".
- 2026-10-01 — And a test that the **event** carries the original, not just the store: bytes stored but never named by
  an event are unreachable by every read *and* by the takedown that would free them, so they would sit in the backup
  scope forever with nothing pointing at them.
- 2026-10-01 — `gofmt` clean, `go build`, full `go test ./...` green. Moving to done.
