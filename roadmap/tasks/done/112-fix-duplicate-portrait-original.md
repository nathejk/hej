# 112 — Fix the retained "original": it was a duplicate of the display image

**Status:** done
**Priority:** high
**Created:** 2026-08-29
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-29
**Completed:** 2026-08-29

## Description

Found by the maintainer on the **production** stack the day after task 111 shipped, by
reading the blob directory and then the event off `NATHEJK`:

```json
"ref":      { "bytes":  74403, "width": 1024, "height": 1024 },
"original": { "bytes": 141970, "width": 1024, "height": 1024, "orientation": 1 }
```

The retained "original" had **identical dimensions** to the display rendition: 1.9× the
storage for zero additional pixels, on the one directory that must be backed up. Task
111's entire justification — keep the original so renditions can be regenerated later
— was not being delivered.

Three causes, all in code I wrote:

1. **`PhotoCapture.vue` downscaled to `MAX_EDGE = 1024`** before upload, which is
   exactly the size the server stores as the display rendition. The server therefore
   never saw anything bigger than what it was already keeping.
2. **The capture constraints asked for a square**: `width: { ideal: 1280 }` *and*
   `height: { ideal: 1280 }`. That fights the sensor's native aspect and produced a
   1024×1024 frame — field of view discarded before the server ever saw it. Visible in
   the event above as a square portrait from a phone camera.
3. **Nothing checked whether the original was worth keeping.** `Prepare` stored it
   unconditionally, so a client that (reasonably) pre-downscaled silently doubled the
   backup.

## Acceptance Criteria

- [x] An original that is **no larger than the display image** is not stored.
- [x] The capture constrains one dimension only, so the device picks its native aspect.
- [x] The client uploads at a size that leaves the original genuinely more detailed
      than the display rendition, without sending a multi-megabyte file over mobile
      data.
- [x] Tests cover same-size, smaller, larger and one-pixel-larger.
- [x] `go test`/`vet`/`staticcheck` green; `npm run type-check`/`build` green;
      **prod image builds** (`--target prod`, which gates on tests + staticcheck under
      `GOWORK=off`).

## Progress Log

- 2026-08-29 — Task created from the maintainer's production evidence. Worth recording
  that the *deduction* came from source (`MAX_EDGE = 1024` caps the upload, so the
  original cannot exceed the display size) and the event only confirmed it — but it
  took a real deployment to notice at all, because every test in tasks 111 used a
  fixture larger than the display limit. That is the gap: the fixtures exercised the
  case the code was designed for and never the case the client actually produced.
- 2026-08-29 — Added the guard in `imaging.Prepare`: keep the original only when its
  longest edge exceeds the display image's. Compared on **pixels, not bytes** — a
  same-size original that merely encodes at higher quality buys a marginal reduction in
  generational loss on a future thumbnail, which is not worth doubling the backup for.
  This makes "keep the original" self-limiting: it now costs storage exactly when it
  buys something.
- 2026-08-29 — Client `MAX_EDGE` 1024 → **2048**. Deliberately not unbounded: this
  uploads over rural mobile data, at night, so ~2048px at q0.85 (a few hundred KB)
  against 1–4 MB for an untouched camera still is the trade. The server's 4 MiB cap
  remains the backstop.
- 2026-08-29 — Capture constraints now name **width only** (`ideal: 1920`). Asking for
  both axes asked for a square; one axis lets the device return its whole frame. Noted
  in the code that this is still a *video* frame — `ImageCapture.takePhoto()` does not
  exist in Safari, so a camera-native still is only reachable through the
  `<input capture>` fallback, which is a product decision (see below), not a bug.
- 2026-08-29 — One existing test had to change, and its failure was the guard working:
  `TestPrepareRecordsOrientationForTheStrippedOriginal` used an 800×400 source against
  a 1024px display, so under the new rule there is no original to carry an orientation.
  Fixture raised to 1600×800, and the reason written into the test so nobody "fixes" it
  back.
- 2026-08-29 — Verified: `gofmt -l`, `go test ./...`, `go vet ./...`,
  `staticcheck ./...`, `npm run type-check`, `npm run build`, and — for the first time —
  **`docker build --target prod`** (20.1 MB image), which runs `go test -timeout 60s
  ./...` + `staticcheck` with `GOWORK=off`. That last one had been unverified across the
  previous push because Docker Desktop was down.

## Expected effect on the next upload

Display stays 1024px. The original should now arrive at up to 2048px on its longest
edge, in the camera's own aspect, and be kept because it is genuinely bigger. If a
device only offers a small frame, no original is stored at all — which is the correct
outcome rather than a duplicate.

Re-check with the same method that found it: read the event, compare `original.width`
against `width`.

## Still open (product decision, not a bug)

A live-preview capture on iOS cannot reach the camera's full resolution. If
archival-quality faces matter more than the in-app face guide, the OS camera
(`<input capture>`) should become the primary button — the server already applies EXIF
orientation for that path (task 104). Raised with the maintainer; unanswered, and not
blocking.
