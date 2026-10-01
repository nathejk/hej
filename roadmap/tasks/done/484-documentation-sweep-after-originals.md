# 484 — Correct the docs that say no original is kept

**Status:** done
**Priority:** medium
**Created:** 2026-10-01
**Picked up by:** agent session (PRD 027 rollout)
**Started:** 2026-10-01
**Completed:** 2026-10-01

**PRD:** 027 (§10 step 9)
**Depends on:** 479

## Description

Three places in the repo state something that becomes **false** on the day PRD 027 ships. Stale documentation is
ordinarily a tidy-up; this is not, because one of these is phrased as an absolute prohibition and somebody will cite
it back at a future change.

**1. `cmd/api/albummedia.go`'s header — this is the one that matters.** It currently says *"**Never** change the
pipeline to preserve EXIF because this feature wants a coordinate."* Left standing, it reads as a rule that was
quietly overruled, which invites the next person to overrule the remainder of it — and the remainder is load-bearing.
So it must be **rewritten to say precisely what survives and what changed, and why**:

- **What survives:** no reader is served unexamined metadata. Every rendition is still re-encoded and stripped. A
  photograph of a child must not carry where it was taken around inside a file nobody has looked at (PRD 003 §6).
  This half did not move an inch, and `TestStoreAlbumImageReadsTheCoordinateAndStripsIt` now tests it explicitly.
- **What changed:** the archive may hold metadata. That half was never argued for separately — it followed from the
  archive and the display image being the same object. Once they are two objects the questions come apart, and an
  archive master that has lost the capture time, the camera and the lens is a worse archive, irrecoverably.
- **What bounds it:** `requireAdmin` is now a privacy boundary and not only an access one, and the guard from task 476
  is what keeps that from drifting.

**2. PRD 022 §8.5** records the old decision — that an admin upload keeps no separate original — as a deliberate
choice. It should not be rewritten as though it never happened: note that PRD 027 supersedes it, and why, so the
reasoning chain stays readable.

**3. `nathejk/table/photo/table.sql`'s header**, "Everything here is rebuildable except the blobs", plus its note
that an admin upload keeps no separate original. The sentence is now more pointed than it was: the original is the one
blob that is **not** rebuildable from anything, which is exactly why task 480 forbids pointing repair at it.

No behaviour changes in this task. Prose only.

## Acceptance Criteria

- [x] `cmd/api/albummedia.go`'s header no longer contains an absolute "never preserve EXIF" rule, and instead states
      what survives (no reader is served unexamined metadata; every rendition is stripped), what changed, and why
- [x] The rewritten header names the boundary that bounds the reversal — `requireAdmin` plus the task 476 guard
- [x] PRD 022 §8.5 records that PRD 027 supersedes it, without erasing the original reasoning
- [x] `nathejk/table/photo/table.sql`'s header is corrected, and says the original is the blob that is not
      rebuildable
- [x] A grep for the old claims across the repo turns up nothing else stating them
- [x] No behaviour or test changes in this task
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created from PRD 027.
- 2026-10-01 — Picked up. The three named places plus whatever a grep turns up.
- 2026-10-01 — `albummedia.go`'s header rewritten. The deleted sentence was *"**Never** change the pipeline to preserve
  EXIF because this feature wants a coordinate"*, and deleting it silently would have been the worst option: it reads
  as absolute, so the next person to meet a related change would cite it from memory. The header now says what the
  prohibition was defending, which half moved and which did not, and states the replacement rule in its place —
  *the original keeps everything and is admin-only; every byte a reader is served is stripped; neither half may be
  relaxed without the other being re-argued*. It also records that a **portrait** original is still stripped, so the
  two are not read as an inconsistency: same question, different subject.
- 2026-10-01 — `photo/table.sql`'s "everything here is rebuildable except the blobs" header rewritten to say which
  column is in which blob class now, **and why `blobRef` used to be the other one**. Kept the explanation that the
  classification tracks "is this the only copy of these pixels", not "was this re-encoded" — that is the sentence which
  makes the glimt asymmetry (R4a) obviously correct rather than looking like an oversight.
- 2026-10-01 — PRD 022: amended §8.4's EXIF bullet in place with a pointer to PRD 027 rather than rewriting it, so the
  original decision is still legible. Also amended §8.5, which turned out to need it for a reason the task had not
  anticipated: it says the id is "the content hash", and since there are now two stored objects that is ambiguous. It
  is the **display rendition's**, necessarily — keying on the original would give one photograph two rows when the same
  file is re-saved or stripped, which is the exact idempotence property §8.5 exists to state.
- 2026-10-01 — **A grep for the old claim found two places the task had not listed**, which is the argument for
  grepping rather than working through a list:
  * `blobclass_test.go`'s header asserted the library was the same case as glimt. Rewritten to say the rule is
    unchanged and only the object it selects has moved.
  * `albumpage.go`'s repair plan said "an album upload keeps no separate original, so the full rendition is the
    source". That comment now carries a **decision**: the display image *is* rebuildable from the original now, but
    this is the public album read and it must not reach for one — a rebuildable rendition is not worth widening the
    boundary that file exists to hold, so a public miss still degrades.
  * `internal/blob/blob.go`'s reference to glimt keeping no original was checked and **left alone**: it is still true,
    and it now reinforces R4a.
- 2026-10-01 — ✅ `gofmt`, `go build`, full `go test ./...` green.
