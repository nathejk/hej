# 444 — Apply the sort: on a mode change, and when photographs are added

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 024 §6 R2/R7 and §8 D1. Needs 443, and 439 for the reload.

The sort **materialises into ordinals** rather than being a read-time `ORDER BY`. `album_item.ordinal` is
already the order both the curator's grid and the public album page read, and `VerbItemsReordered` already
means "here is the album's whole order" — so sorting is that event with the list computed by a rule
instead of by a pointer. Every read stays as it is, and the public page agrees with the curator's by
construction.

Two moments recompute:

- `PATCH /api/admin/albums/{albumId}` sets a non-manual `sortMode` — reject an unknown value with a 400
  rather than ignoring it;
- `POST /api/admin/albums/{albumId}/items` adds to an album whose mode is not `manual`. This is the
  maintainer's "applied when added to album". The response should say whether it re-sorted, so the tool's
  note can say so.

**Both endpoints already have OpenAPI annotations, which must be updated** — including the add endpoint's
paragraph about ordinals being appended after the maximum, which stops being the whole truth here.

The interaction with a removed membership is the subtle part: `NextOrdinal` exists so a removed position
is never reused (it would resurrect the row's soft delete through the upsert). A full resort assigns
positions to **live items only**, so check what that does to removed rows before writing it.

## Acceptance Criteria

- [ ] A mode change recomputes and publishes one `itemsreordered`
- [ ] Adding to a non-manual album recomputes; adding to a manual one appends exactly as today
- [ ] An unknown `sortMode` is a 400
- [ ] A resort cannot resurrect a removed membership
- [ ] OpenAPI annotations updated on both endpoints
- [ ] The tool waits for the projection before reloading (task 439)

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R2/R7, §8 D1.
