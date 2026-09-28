# 444 — Apply the sort: on a mode change, and when photographs are added

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

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

## What changed

`applyAlbumSortMode(year, albumID, mode, items)` publishes one `itemsreordered` event carrying the album's
whole new order, and reports whether it published anything. Called from two places and no others: a
`sortMode` change on `PATCH /api/admin/albums/{albumId}`, and `POST …/items`.

It **declines** in three cases, each for a stated reason: `manual` (which is what makes that mode free), an
empty album, and — the one worth keeping — **an album already in the order the mode asks for**. An add that
lands at the end of a time-sorted album is the common case, and publishing an event that moves nothing would
be noise on a log that is never rewritten and would make `resorted` a lie.

Both endpoints' OpenAPI descriptions updated. The add endpoint's paragraph about ordinals being appended after
the maximum is no longer the whole truth and now says so. (It also had a **duplicated closing paragraph**,
introduced by an earlier edit; removed.)

## The awkward part, and it was unavoidable

On an add, the newly added photographs' sort keys **cannot** come from the album read: the item-added events
have just been published and the projection is downstream of the log, so re-reading the album returns the
album as it was. `albumItemsForSort` therefore fetches them from the library — `photo.Filter.PhotoIDs`, the
presence filter task 438 added for the uploader — chunked at 200 to match `maxAdminLibraryIDs`, since a bulk
add is bounded by `maxAdminSelection` (2000).

`IncludeDeleted` on that read, because a curator can file a photograph that is deleted from the library: the
membership exists and the position has to be sorted like any other. Without it the photograph would come back
with no keys and sort as if it had none — a different answer depending on whether somebody had deleted it.

## The failure mode this exposed

A nil `PhotoCurator` panicked the add handler, found by the existing test suite. Not merely a test gap: the
library being unreadable is a real state every other handler checks for. Three possible answers, and the two
obvious ones are both wrong:

- **failing the request** would tell the curator their selection was not filed, when it was — the item-added
  events are already on the log;
- **sorting without the new photographs** would publish an order that does not name them, and the fold vacates
  every position before placing the named ones, so the additions would land in the offset range at the end of
  the album in an order nobody chose.

So the re-sort is **skipped**, `resorted` is false, a warning is logged, and the next add or mode change puts
it right. Stale, not wrong — the same degradation `readAdminLibraryPage` already chooses when it cannot answer.

## Acceptance Criteria

- [x] A mode change recomputes and publishes one `itemsreordered`
- [x] Adding to a non-manual album recomputes; adding to a manual one appends exactly as today
- [x] An unknown `sortMode` is a 400 (task 442)
- [x] A resort cannot resurrect a removed membership
- [x] OpenAPI annotations updated on both endpoints
- [ ] The tool waits for the projection before reloading — **task 446's work**; `ctx.settled` exists (439)

## Progress Log

- 2026-09-28 — Picked up. Checked the `itemsreordered` fold first, for the removed-ordinal hazard the task
  flagged: it vacates **every** position including removed ones and then places only the named ones, so a
  removed row keeps an offset ordinal and its soft delete is untouched. A resort cannot resurrect one. This is
  established behaviour from the manual reorder, not something applying the sort introduces.
- 2026-09-28 — Ordered the mode change as update-then-resort. A failure then leaves the album with the mode the
  curator chose and a stale order, which the next add corrects; the other way round would leave an album
  sorted by a rule it does not claim to follow, which nothing would ever correct.
- 2026-09-28 — Hit the nil-`PhotoCurator` panic and chose the degradation above.
- 2026-09-28 — ✅ Six behavioural tests, including the unreadable-library case. Mutation-checked: removing the
  already-in-order check publishes a needless event and fails; sorting without the added ids publishes nothing
  and fails.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean.
