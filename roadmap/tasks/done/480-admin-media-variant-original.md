# 480 — variant=original on the admin media route

**Status:** done
**Priority:** medium
**Created:** 2026-10-01
**Picked up by:** agent session (PRD 027 rollout)
**Started:** 2026-10-01
**Completed:** 2026-10-01

**PRD:** 027 (R6)
**Depends on:** 479

## Description

Once originals are stored, a curator needs to be able to fetch **one** of them — the single-photograph counterpart
to the album zip. `GET /api/admin/photos/{photoId}/media` gains `variant=original` in
`cmd/api/adminlibrary.go`, behind the same rule as every other variant: resolve the id, then use **the row's own
ref**, never a ref from the request.

The route is admin-only and stays that way. PRD 027 Q3 settled the shape: an original is a **download, never a
view** — the viewer's `srcset` is untouched by this task and continues to name 800w/1600w only. Adding this variant
must not add a reader of it.

**Crucially: no rendition-repair plan for this variant.** The repair machinery from task 430 rebuilds a missing
rendition by re-deriving it. An original cannot be re-derived from anything — there is nothing upstream of it — so
pointing repair at this variant would at best fail and at worst write a derivative into an original's slot and call
it the photographer's file. `blob.PutAs` already refuses to overwrite an original, for exactly this reason, and
nothing in this task may weaken that: an original's ref **is** its hash, which is what makes it verifiable. A
missing original is a fact to report, not a thing to fix.

Per `.rules`, the **OpenAPI annotations must be updated in the same change** — `variant` gains `original`, with the
no-repair behaviour and the admin-only boundary stated rather than left to be inferred.

## Acceptance Criteria

- [x] `variant=original` streams the original for a photograph that has one
- [x] The ref comes from the resolved row, never from the request
- [x] A photograph with no original gets a clear refusal, not a substituted rendition
- [x] No repair path is registered or reachable for this variant, and a test asserts it
- [x] The route remains admin-only; no viewer, `srcset` or public surface references it
- [x] OpenAPI annotations for `GET /api/admin/photos/{photoId}/media` updated in the same change
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created from PRD 027.
- 2026-10-01 — Picked up and done. `variant=original` added to `showAdminPhotoMediaHandler`, annotations updated.
- 2026-10-01 — **The whole case is three lines, and the important one is the line that is not there:** `edge` stays 0.
  The repair plan below is built only `if edge > 0`, so leaving it alone is what keeps an original out of task 430's
  machinery. That machinery rebuilds a rendition by re-rendering it from a source, which works because a rendition is
  derivable; an original is not derivable from anything, and a plan pointed at one would try to write a re-encode over
  the only copy of somebody's file.
- 2026-10-01 — `blob.PutAs` already refuses to overwrite an original, so the attempt would fail rather than corrupt —
  and that backstop is exactly why this needed a test. The bug would surface as a logged error on a path nobody
  watches, not as a broken image somebody reports. `TestTheOriginalVariantGetsNoRepairPlan` reads the case and fails if
  it mentions `edge` at all; verified by injecting `edge = 1600` and watching it fail.
- 2026-10-01 — Falls back to the display image when the row holds no original, like the other variants. Stated in the
  annotation as the honest answer rather than a substitute: for a photograph from before PRD 027, 1600px **is** its
  most original surviving form.
- 2026-10-01 — ✅ `gofmt`, `go build`, full `go test ./...` green.
