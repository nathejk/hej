# 111 — Keep the original upload (metadata stripped) so renditions can be regenerated

**Status:** done
**Priority:** medium
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

Maintainer request following task 110: **also save the original.**

The reason is sound and is one task 110 wrote down as a limitation: renditions are
produced *at upload*, so with only a 1024px re-encode on disk, adding a thumbnail
size — or wanting more pixels on a face at 03:00 — can only ever apply to portraits
taken after the change. The original is what makes a backfill possible at all.

**But it collides with a documented safeguard.** PRD 003 §6 requires that the app
"re-encodes rather than trusting the uploaded bytes, and strips EXIF (notably GPS)".
For the display renditions that is free — they are re-encoded from pixels. An
original cannot be re-encoded without defeating the point of keeping it, and a phone
photograph routinely carries the location it was taken. For a photograph of a child
at a scouting event, that is precisely the field that must not be retained.

So the original is stored **losslessly de-metadata'd**, not verbatim: whole EXIF /
XMP / ICC / comment blocks are dropped and the compressed pixel data is copied
unchanged. Same pixels, no provenance.

## Acceptance Criteria

- [x] The original is stored content-addressed, at full resolution, alongside the
      renditions.
- [x] All metadata blocks are removed; the pixels are provably unchanged.
- [x] The EXIF **orientation** is recorded in the event and the row, since stripping
      removes it from the file and a re-render would otherwise not know which way up
      the face goes.
- [x] The original is **never served** to a client.
- [x] Retention deletes it with everything else.
- [x] Formats with no metadata scrubber keep no original rather than storing
      something unexamined.
- [x] Configurable, because the storage consequence is large.
- [x] Tests, incl. pixel-identity after stripping; `go test`/`vet`/`staticcheck`
      green.

## Progress Log

- 2026-08-28 — Task created from the maintainer's "we should also make sure to save
  original".
- 2026-08-28 — Flagged the conflict with PRD 003 §6 rather than silently overturning
  it, and resolved it by scrubbing instead of re-encoding. `imaging.StripMetadata`
  drops JPEG APPn+COM segments and copies the entropy-coded scan verbatim; for PNG it
  keeps a **chunk allowlist** (critical + the ancillary ones that affect how pixels
  are interpreted) and stops at IEND, which also removes anything appended after it —
  a favourite hiding place, since decoders ignore it. An allowlist, not a blocklist:
  a blocklist has to enumerate every metadata chunk that exists now *and* every one
  added later.
- 2026-08-28 — Pixel identity is asserted, not assumed: a test decodes the upload and
  the stripped original and compares **every pixel**. That is the property that makes
  this lossless rather than a re-encode, and it is the one a careless change would
  break invisibly.
- 2026-08-28 — **Orientation had to move into the log.** Stripping removes the tag,
  and the display renditions have the rotation baked in — so without recording the
  number, the stored original would be the one artefact that had *lost* information.
  It now travels in `PortraitCaptured.original.orientation` and in
  `person.portraitOrientation`: metadata we control and can audit, rather than
  metadata riding along inside a file. A test asserts the stripped file reports
  orientation 1, so nothing can rotate it twice.
- 2026-08-28 — **The original is never served.** No `size=original`, deliberately: the
  renditions are re-encoded from pixels and are therefore provably images, while the
  original is bytes we merely decoded once. Keeping it off the response path removes
  the polyglot-file question entirely. Adding it later is a small change if a real
  need appears — I did not invent one.
- 2026-08-28 — GIF and anything else keeps **no** original: there is no scrubber for
  it, and the whole point is that nothing unexamined is retained. Every camera path
  produces JPEG in practice.
- 2026-08-28 — A malformed original ref costs the original, not the portrait — the
  consequence is that one member cannot be backfilled later, which beats losing their
  photo. A *storage* failure, by contrast, fails the upload: silently dropping it
  would leave one member quietly un-backfillable, discovered only when a future
  re-render skipped them for no visible reason.
- 2026-08-28 — Retention: the original is included in `Person.PortraitRefs()`, which
  is the single definition of "every object this portrait occupies" that task 110
  introduced. Without that, this task would have left a **full-resolution** face on
  disk after the record said the portrait was deleted.
- 2026-08-28 — **Storage consequence, stated plainly because it is the real cost.**
  The blob store is the only thing here that cannot be rebuilt from the log and
  therefore the only thing that must be backed up (PRD 008 §8). Renditions are ~15 KB
  per member; an original is 1–4 MB. For ~800 participants that is roughly **12 MB →
  ~2 GB** — two orders of magnitude on the backup. Hence
  `PORTRAIT_KEEP_ORIGINAL` (default **true**), so an operator who cannot afford it
  can say so without a deploy.
- 2026-08-28 — **Verified live** with a generated 1600×1200 JPEG carrying EXIF
  orientation 6 and a `GPSLatitude=56.1234 GPSLongitude=9.5678` string inside the
  EXIF block:
  * event body: `"ref":…,"width":768,"height":1024` (display, **rotated upright**),
    `"thumbs":[{"name":"thumb256",…,"width":192,"height":256}]`, and
    `"original":{"ref":"569ff01b…","bytes":40718,"width":1600,"height":1200,"orientation":6}`
    — i.e. the sensor's unrotated pixels plus the number needed to re-render them;
  * upload was 40,793 B, stored original 40,718 B — the 75 bytes removed being exactly
    the metadata;
  * `grep GPSLatitude` and `grep Exif` on the stored blob: **0 matches**;
  * the blob still decodes: `JPEG image data, baseline, 1600x1200`;
  * row carries `portraitOriginalRef` + `portraitOrientation=6`.
- 2026-08-28 — Then ran the purge with `PORTRAIT_RETENTION=1s`: `/blobs` went to
  **0 files** (display, thumbnail *and* original) and the row's refs and orientation
  were cleared.
- 2026-08-28 — ✅ All criteria met. `gofmt -l`, `go test ./...`, `go vet ./...`,
  `staticcheck ./...` green.

## Follow-up worth knowing

- **HEIC.** iOS can hand a `.heic` file to a plain file input. Go cannot decode it, so
  such an upload is rejected as "not an image" — with or without this change. Noted
  for task 108's device pass; if it turns out to happen, the fix is a client-side
  canvas conversion in the fallback path, not a server-side decoder.
