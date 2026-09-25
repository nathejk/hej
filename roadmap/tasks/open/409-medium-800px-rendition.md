# 409 — An 800 px rendition, and the delete walks that have to know about it

**Status:** open
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `photo.mediumRef` exists with the `thumbRef` shape and the documented may-be-empty rule, in both
      `table.sql` and an `EnsureColumn` migration, verified against a live `describe photo`
- [ ] An upload produces the 800 px rendition; the event carries the ref, the fold writes it, and
      `album.Item` and `photo.LibraryPhoto` expose it
- [ ] `variant=medium` works on both media routes, falls back to the full image when the ref is empty, and both
      OpenAPI annotations are updated
- [ ] `KEY medium_lookup (mediumRef)` exists, and the shared-blob delete walks include `mediumRef` — proven by
      break-test, i.e. with the column omitted a shared medium blob is wrongly purged or wrongly orphaned
- [ ] The `glimtThumbEdges` decision (shared or split) is recorded in PRD 023 and reflected in the code, so
      glimt's storage does not change by accident
- [ ] No backfill is run and no photograph without a medium rendition renders a gap

## Progress Log

- 2026-09-25 — Task created from PRD 023.
