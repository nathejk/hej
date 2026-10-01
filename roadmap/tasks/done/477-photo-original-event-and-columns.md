# 477 — Carry the original on photo.Uploaded and in the projection

**Status:** done
**Priority:** high
**Created:** 2026-10-01
**Picked up by:** agent session (PRD 027 rollout)
**Started:** 2026-10-01
**Completed:** 2026-10-01

**PRD:** 027 (R2, R3)
**Depends on:** 476

## Description

Before anything can store a photographer's file, the log and the projection need somewhere to say that one exists.
This task is that shape change and nothing else: no upload path work, no blob writes, no UI. It lands first so that
task 479 has a field to publish into, and so the shape can be reviewed on its own rather than inside a change that
also rewrites the ingest pipeline.

**The event (R2).** `photo.Uploaded` gains an optional nested `Original` object carrying ref, content type, bytes,
width and height — deliberately mirroring `person.PortraitOriginal` in `nathejk/table/person/portrait.go`, which is
the same idea already solved once in this codebase. Read that type before writing this one; matching it means a
reader who knows one knows the other, and it is the reason the shape is not up for invention here.

**Absent is a normal value, not an error.** Every `photo.Uploaded` event already in the log has no original and
never will — there is no source to backfill from (PRD 027 §4). So the field is optional in the sense that nothing
downstream may treat its absence as corruption, a missing migration, or something to repair.

**The projection (R3).** `photo` gains `originalRef`, `originalWidth`, `originalHeight`, `originalBytes` and
`originalContentType`, **all with defaults** — empty string, zero — so that the existing rows fold unchanged and
**no migration is required**. The projection is replayed; an old event simply produces the default values.

**The fold validates the ref and blanks it rather than failing.** Use the existing `validRef` in the `photo`
package. A ref that does not pass is written as empty, and the fold continues. Failing the fold would let one
malformed event stop the projection for the whole library, which is a far worse outcome than one photograph whose
original is unreachable — and the row is still perfectly serviceable, because every reader is served a rendition
anyway (PRD 027 R11).

`LibraryPhoto` gains the matching fields, because every consumer of the projection reads through it: the zip
(task 481), the media route (task 480) and the admin tool's "holds an original" statement (task 482) all need the
ref and the dimensions without a second query.

Scope is `nathejk/table/photo/`. Nothing in this task writes a blob or changes a response.

## Acceptance Criteria

- [x] `photo.Uploaded` carries an optional nested `Original` with ref, content type, bytes, width and height, shaped
      as `person.PortraitOriginal`
- [x] The five columns exist with defaults such that replaying the existing log produces unchanged rows, and no
      migration file is needed
- [x] An event with no `Original` folds to empty/zero values and is not treated as an error anywhere
- [x] The fold runs the ref through the existing `validRef`; an invalid ref is stored as empty and the fold does not
      fail
- [x] `LibraryPhoto` exposes the five fields
- [x] A test folds an `Uploaded` with an original, one without, and one with an invalid ref, and asserts all three
      outcomes
- [x] Full gate clean: `gofmt`, `go vet`, `GOWORK=off go test ./...`
- [x] **Added during the work:** the five columns are written as **one group**, guarded on `originalRef`, so a
      re-upload cannot blank an original the row already holds. This was not in the original criteria and is the most
      consequential line in the task — see the log.
- [x] **Added during the work:** `original_lookup` index on `originalRef`, in both `table.sql` and the
      `EnsureIndex` drift list. Every other ref column has one because `RefsInUse` interrogates it inside a delete
      path; this one will be interrogated inside the *takedown* (task 478).

## Progress Log

- 2026-10-01 — Task created from PRD 027.
- 2026-10-01 — Picked up. Plan: `Original` on the event mirroring `person.PortraitOriginal`, five columns in
  `table.sql` + the `EnsureColumn` drift list, fold, `LibraryPhoto` fields and scan.
- 2026-10-01 — Event shape added in `events.go`. Mirrors `PortraitOriginal` with **one deliberate difference**: no
  `Orientation` field. The portrait records it because stripping metadata removes the tag, and without it a future
  re-render would not know which way up the face goes. PRD 027 does not strip, so the tag is still in the file and a
  column for it would be a second, drift-prone copy of something the bytes already say.
- 2026-10-01 — **The real finding of this task, and it was not in the brief.** The fold's existing "fill a gap, never
  open one" rule (for `shotAt` and `fileName`) turns out to be exactly what the original needs, for a much more
  serious reason. `photoId` is the hash of the stored **display** rendition, not of the upload — so the same id
  legitimately arrives from two different files: a stripped copy, or one re-saved by an editor, produces the same
  1600px re-encode. Re-uploading is the *documented* recovery procedure for a half-failed batch (task 372), so this
  is routine. Under a plain `VALUES(...)` upsert — which is how every rendition in that clause is written, because
  losing one is survivable — a re-upload would have **blanked the original**, i.e. destroyed the only copy of the
  photographer's file. Fixed by guarding all five columns on one condition, `VALUES(originalRef)=""`.
- 2026-10-01 — Guarded on `originalRef` rather than each column on its own value, deliberately: per-column guards
  would let a new ref land beside the previous file's dimensions — a row describing a photograph that does not exist,
  and an original is the one thing here with nothing to check it against. All five move or none do.
- 2026-10-01 — Added `original_lookup` on `originalRef` to `table.sql` and the `EnsureIndex` list. Noticed from the
  existing comment "**one key per ref column, and every ref column needs one**" — `RefsInUse` will interrogate this
  one inside the library takedown (task 478), where being slow is the least of the ways to be wrong.
- 2026-10-01 — ✅ Tests in a new `originalfold_test.go` rather than appended to `consumer_test.go`: the rule being
  pinned is "a later event must not destroy the only copy of somebody's file", which deserves to be findable by name.
  Five cases, including one asserting that **no event other than `Uploaded`** writes these columns — `MediumAdded`
  exists because a rendition can be produced later, and an original cannot.
- 2026-10-01 — Verified the important test is not vacuous: replaced the guarded clause with a plain
  `originalRef=VALUES(originalRef)` and `TestUploadedNeverBlanksAnOriginalItAlreadyHas` failed with both of its
  messages. Reverted; `git diff --stat` confirms only the intended 46 insertions.
- 2026-10-01 — `gofmt` clean, `go build ./...`, full `go test ./...` green. Moving to done. Next: task 478 —
  `RefsInUse` and the takedown, which must land before 479 so no original can exist that a takedown would miss.
