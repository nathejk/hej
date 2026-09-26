# 409 — An 800 px rendition, and the delete walks that have to know about it

**Status:** done
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent
**Started:** 2026-09-26
**Completed:** 2026-09-26

## Description

PRD 023 §7.9 and §8 ("Data / storage"). Maintainer, 2026-09-25: *"if a small screen then don't fetch original
image — a medium size would do"*. The display image is 1600 px and ~200–400 KB, while a 390 pt phone can show
about 800 px of it at 2× — four times the pixels for no visible gain, times however many photographs somebody
swipes through. So: a third rendition at **800 px**, joining the 1600 px display image and the 320 px
thumbnail. **This is the only schema change in PRD 023.**

A rendition at upload, **not** a resize on request. `imaging.Prepare` already takes a *list* of thumbnail edges
and the loop over them is already there, so producing an 800 px variant is a one-element change in that slice.
There is no image-resizing endpoint in this service and this is not the feature that should introduce one.

### What it touches

- `photo.mediumRef VARCHAR(64) NOT NULL DEFAULT ""` — the same shape as `thumbRef`, **including the documented
  "may be empty, readers fall back to the full image" rule**. That rule is what removes the need for a
  migration, so write it down rather than implying it.
- The upload event gains the ref and the fold writes it.
- `album.Item` and `photo.LibraryPhoto` carry it.
- `variant=medium` on **both** media routes — `/api/public/albums/{albumId}/media/{ordinal}` and
  `/api/admin/photos/{photoId}/media` — with their **OpenAPI annotations updated** (§8's endpoint table says so
  for both; `.rules` requires it).
- `KEY medium_lookup (mediumRef)`.

### The part that would bite

**The shared-blob delete walks must be taught about the new column.** `glimtRefsOf`, `refsUsedElsewhere` /
`blobRefsInUse` and the album and photo delete paths enumerate *every* column that can name a blob, because
identical bytes are one object and a delete therefore has to ask every table — task 368's reasoning, which
applies here verbatim. A ref column those walks do not know about has exactly two possible outcomes, and both
are bad: an object orphaned on disk forever, or — worse — a **live object deleted because nothing claimed it**.
Neither shows up in a test that only exercises uploads.

### Two decisions to take explicitly, not by omission

1. **`glimtThumbEdges` is shared with glimt** (`albummedia.go` reads it today). Changing it to `[]int{800, 320}`
   silently changes a second feature's storage, which is the kind of change nobody reviews. **Decide, in
   writing, whether glimt gets the 800 px rendition too or whether the constant splits** — PRD 023 §11's open
   question 8 notes that "both" may well be right, but it belongs to whoever owns PRD 019 rather than to a
   silent constant edit here. Record the decision in the PRD, not only in a commit message.
2. **No backfill is in scope** (§4, §7.9). Every photograph uploaded before this ships falls back to the display
   image, which is correct rather than degraded. If a backfill is ever wanted, §7.9 notes the cheapest moment is
   now, while PRD 022 is in `doing/` and the library holds test data — that is §11's question 9 and stays open.

One process note from task 393 that applies to any new column on an existing projection: `table.sql` is
`CREATE TABLE IF NOT EXISTS`, so adding a column there does nothing to a database that has already booted, and
the `cmd/api` suite runs against stubs that never touch a real schema. **Both halves are needed** — the table
definition and `cqrs.EnsureColumn` — and only a live `describe photo` will tell you.

Task 410 is what makes the browser actually ask for this rendition.

## Acceptance Criteria

- [x] `photo.mediumRef` exists with the `thumbRef` shape and the documented may-be-empty rule, in both
      `table.sql` and an `EnsureColumn` migration — **schema and migration written and guarded by test;
      `describe photo` against a live database is the one step that cannot be taken from here**
- [x] An upload produces the 800 px rendition; the event carries the ref, the fold writes it, and
      `album.Item` and `photo.LibraryPhoto` expose it
- [x] `variant=medium` works on both media routes, falls back to the full image when the ref is empty, and
      both OpenAPI annotations are updated
- [x] `KEY medium_lookup (mediumRef)` exists, and the shared-blob delete walks include `mediumRef` — proven
      by break-test
- [x] The `glimtThumbEdges` decision (split) is recorded in PRD 023 §11 Q8 and reflected in the code, so
      glimt's storage does not change by accident
- [x] No backfill is run and no photograph without a medium rendition renders a gap
- [x] The album page emits `data-medium` **in this task**, and only for photographs that have one

## What was built

### The column, and the two halves a new column needs

`photo.mediumRef VARCHAR(64) NOT NULL DEFAULT ""`, the `thumbRef` shape exactly. The may-be-empty rule is
written into `table.sql` rather than implied, because that rule *is* the no-backfill story: "" means "serve
the full image", so every photograph uploaded before this ships keeps working untouched.

Both halves, per task 393's process note: the column is in `table.sql` **and** in `table.go`'s
`EnsureColumn` list, because `CREATE TABLE IF NOT EXISTS` does nothing to a database that has already
booted and the `cmd/api` stubs never touch a real schema.

**A third half turned out to be needed.** `cqrs.EnsureColumn` adds columns, not keys — so on an existing
database `medium_lookup` would simply not exist, and the purge check would scan the year's photographs
from inside a delete path. Added `ensureIndex` alongside it (same `INFORMATION_SCHEMA` approach, additive
only, keyed on the index name so a hand-added key under another name is left alone). This was not in the
task's list and is the kind of thing only a live `describe photo` would otherwise have found.

### The part the task said would bite, and it did

`photo.RefsInUse` is what decides whether a blob may be deleted. It interrogated two ref columns with
hand-counted placeholder arithmetic (`len(refs)*2`, two copies of the args, two `IN (...)` clauses spelled
out). Adding a third that way is exactly how the next one gets forgotten, so the columns are now **one
list** and the clause, the placeholders and the bound arguments are all derived from it. A missing column
here is not a wrong answer in a stub — it is a live object deleted because nothing claimed it, and content
addressing makes sharing the expected case rather than a corner one (PRD 011 §0b.2).

`admindelete.go`'s ref list gained `MediumRef` on the same terms. Both halves have to know: a rendition
missing from the *purge list* is bytes orphaned forever, a rendition missing from *`RefsInUse`* is somebody
else's photograph blanked.

### The constant split — PRD 023 §11 Q8's narrow half

`glimtThumbEdges` was shared with glimt, so `[]int{800, 320}` would have changed a second feature's storage
for every future upload, silently, in a diff about album pages. Split instead: `libraryThumbEdges` is the
library's and `glimtThumbEdges` stays glimt's, unchanged.

Recorded in PRD 023 §11 Q8 rather than only here, with a note that the question is now *cheaper* than when
it was asked: since task 429 an 800 px rendition is **cache** class — derivable from the display image,
outside the backup scope, rebuilt on demand by task 430 — so the original objection to giving glimt one
("it grows the only irreplaceable data in the service") no longer applies. Whether glimt gets one is still
PRD 019's call.

### By name, never by index

`prepared.Thumbs` is in the order of `libraryThumbEdges`, so the old `prepared.Thumbs[0]` would have
silently re-pointed the *thumbnail* to the 800 px rendition the moment a size was prepended. New
`storeRendition` helper resolves by `imaging.ThumbName(edge)`. That bug would have presented as a layout
problem — the grid quietly serving 800 px tiles — with nothing erroring.

### `data-medium`, conditionally

Emitted only when the photograph has the rendition. An attribute that was always present would name a URL
the server answers by *falling back* — correct bytes under an `800w` label that is a lie, which is the one
thing a `srcset` candidate must not be.

**A boolean, not the ref.** The template ranges over `publicAlbumItem`, a view model that deliberately
carries no blob refs: a content hash in a public payload is a forwardable, unrevokable capability, which is
why the page addresses photographs by ordinal. So the view model gained `HasMedium bool`. Found by the
album page tests failing wholesale when `{{if .MediumRef}}` referenced a field the view model does not
have — the right failure for the right reason.

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-26 — Picked up together with task 410, which was delegated in parallel (disjoint write scope:
  the viewer's own assets and tests).
- 2026-09-26 — Schema, `EnsureColumn`, event field, fold, `Photo`, `LibraryPhoto` and `album.Item` done.
  Noted while writing it that `EnsureColumn` does not add keys, so added `ensureIndex` — otherwise
  `medium_lookup` exists only on fresh databases and the purge check scans on every real one.
- 2026-09-26 — Rewrote `RefsInUse` around a single column list rather than adding a third hand-counted
  placeholder set. ✅ Break-test: dropping `mediumRef` from that list fails
  `TestRefsInUseInterrogatesEveryRefColumn`, mutation-checked.
- 2026-09-26 — Decision recorded: **the constant splits.** PRD 023 §11 Q8 updated, including that tasks
  429/430 make the glimt half of the question cheaper than when it was posed.
- 2026-09-26 — Replaced `prepared.Thumbs[0]` with a by-name lookup and a test that forbids the index,
  because the order dependency is invisible until a size is added.
- 2026-09-26 — `variant=medium` on both media routes, with repair plans (task 430) so a missing 800 px
  rendition rebuilds itself rather than permanently falling back. OpenAPI annotations updated on both.
- 2026-09-26 — `data-medium` on the tile. Album page tests failed wholesale first: the template ranges over
  a view model that holds no refs by design, so this became `HasMedium bool` — which is the better shape
  anyway, since a hash must not reach public HTML.
- 2026-09-26 — Tests in `nathejk/table/photo/medium_test.go` (fold, and source-level guards on the SQL,
  following `album/querysafety_test.go`'s reasoning about `cqrs.Reader` being unfakeable) and
  `cmd/api/medium_test.go` (three sizes decoded from the actual bytes, the empty-ref fallback with an honest
  ETag, the conditional attribute, no ref in the HTML, both renditions stored as cache, and glimt's storage
  unchanged).
- 2026-09-26 — `gofmt`, `go vet` and the full suite clean. All criteria met except the live `describe photo`,
  which needs a real database; the code path for it is written and guarded.
