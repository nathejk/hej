# 303 — POST /api/glimt/media — upload, validate, strip, variants

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `POST /api/glimt/media` accepts a multipart `media` field, returns `{ref, thumbRef, width, height}`
- [ ] EXIF/GPS stripped — test asserts a GPS-tagged input produces output with no EXIF
- [ ] Two variants stored in the blob store
- [ ] 413 on oversize, 400 on undecodable, 401 unauthenticated
- [ ] Registered in `routes.go` behind `requireAuth`
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
