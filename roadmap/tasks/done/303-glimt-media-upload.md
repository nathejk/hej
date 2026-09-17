# 303 — POST /api/glimt/media — upload, validate, strip, variants

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 019 §8. One media item per request, returning a blob ref the client then references when
it creates the glimt.

Reuses what the portrait feature already built (PRD 003): `multipart/form-data`,
`http.MaxBytesReader`, `go/internal/imaging` for decode-validate → EXIF-orient → **strip
metadata** → re-encode, and `go/internal/blob` for content-addressed storage. Follow
`readPortraitUpload` / `normalizePortrait` in `go/cmd/api/photo.go`.

Images only in this task — video is task 322.

Two variants stored: a feed-sized image and a thumbnail. The original is not served.

Failure codes match `/me/photo`: 400 not a decodable image, 413 too large, 429 rate limited,
503 stream unavailable.

## Acceptance Criteria

- [x] `POST /api/glimt/media` accepts a multipart `media` field, returns `{ref, thumbRef, width, height}`
- [x] EXIF/GPS stripped — test asserts a GPS-tagged input produces output with no EXIF
- [x] Two variants stored in the blob store
- [x] 413 on oversize, 400 on undecodable, 401 unauthenticated
- [x] Registered in `routes.go` behind `requireAuth`
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 15:05 — Picked up. Most of this is the portrait's path reused: `internal/imaging`
  decode-validate → orient → re-encode (which is what strips EXIF/GPS) and `internal/blob` for
  content-addressed storage. Only the differences are worth new code.
- 2026-09-17 15:15 — Four constants deliberately differ from the portrait's, each with a reason
  in the file: **12 MiB** upload (a scene, not a face, and less likely pre-cropped);
  **1600px** display edge (a glimt fills a screen and opens full-screen, where 1024 is visibly
  soft); **320px** thumbnail — the single most load-bearing constant in the feature, since the
  hold-collection grid pulls these by the thousand over a congested network; and a **5-minute**
  read deadline (12 MiB at the ~50 KB/s a field at night actually provides is four minutes, and
  the 30s server default would abort the read partway).
- 2026-09-17 15:25 — Decision: **no original is kept**, unlike the portrait (task 111). The
  arithmetic decides it. A portrait is one object per member per event; glimt are unbounded — ten
  items a post, many posts a member — and the blob store is the only thing here that cannot be
  rebuilt from the stream, so it is the only thing that must be backed up. Keeping originals would
  roughly double the only irreplaceable data in the service to enable a re-render nobody has asked
  for. If sharper media is ever wanted, raise `maxGlimtEdge` for new uploads.
- 2026-09-17 15:30 — Second deliberate inversion: **a thumbnail failure does not fail the
  upload**, where for a portrait it does. PRD 007 needs every portrait to have a thumbnail, so an
  incomplete rendition set there is a state to handle forever. Here the client already falls back
  to the full item when `has_thumb` is false (task 302), so a lost thumbnail costs one grid tile
  some bandwidth rather than costing the member their photo.
- 2026-09-17 15:35 — Field is `media`, not `photo`: it carries video from task 322, and a field
  named for one of the two would be a lie in half the requests. A test asserts the error names the
  right field, since a client author will reach for `photo` by analogy.
- 2026-09-17 15:40 — Separate `glimtMediaLimiter` (60/hour) rather than sharing `photoLimiter`
  (10/hour). The actions are nothing alike: two full posts would exhaust a portrait budget, or a
  portrait limit loose enough to be pointless. Task 311 revisits it next to the storage ceiling,
  which is the limit that actually protects the disk.
- 2026-09-17 15:50 — **🐞 Found and fixed a silent data-loss bug** while writing the ref-validation
  test, which failed on a case I had written as a should-reject. Three facts combined badly:
  `blob.Ref.Valid` accepts **uppercase** hex (correct for its own job — deciding what is safe to
  turn into a filesystem path), `blob.ComputeRef` only ever emits **lowercase**, and the glimt
  projection's `validRef` only accepts **lowercase**. So an uppercased ref would pass upload
  validation, travel on the created event, and then be **dropped by the fold** — a three-photo
  glimt arriving with two, nothing logged, nothing for the member to retry. Fixed by requiring the
  canonical form at the boundary (`glimtBlobRef`) rather than loosening the projection: lowercase
  is what the store actually produces, and a projection accepting both spellings would be one
  where two rows can reference one object. `blob.Ref.Valid` is left alone — other callers rely on
  its path-safety contract.
- 2026-09-17 15:55 — Also tested the oversize case on **both transports** from the start. The
  portrait endpoint shipped with only the raw path covered and reported an 11.6 MB multipart
  upload as a 400 "missing field" — doubly wrong, and found live (see photo.go). The multipart
  branch here handles it.
- 2026-09-17 16:00 — ✅ All criteria met. `gofmt` clean, `go build ./...`, full `go test ./...`
  green. Moving to done.

### ⚠️ Finding for task 304: there is no `/api/glimt/version` to add

`routes.go` carries an explicit instruction above `/api/sync`:

> The multiplexed freshness check (PRD 017): one request answers "did anything I hold change?" for
> every dataset this caller has. It replaced a per-dataset version endpoint (`/api/contacts/version`,
> retired in task 292) — **do not add another one; add a key here.**

PRD 019 §8 asks for `GET /api/glimt/version`, which was written before that convention existed in
the PRD author's view. Task 304 should instead add a **`glimt` key to `app.syncDatasets()`** backed
by the `Version()` querier already written in task 301. This is the repo convention winning over the
PRD text, and it is also just better: six endpoints would be six requests per foreground from a few
hundred devices on a mobile link. I will update PRD 019 §8's endpoint table when 304 lands.
