# 443 — The sort itself

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 024 §6 R2/R4/R6. Needs 441 and 442.

One function from (the album's live items, their photographs, the mode) to an order — the same shape
`moveAlbumOrder` has, and testable the same way: a list in, a list out, no database.

- `time-asc` / `time-desc` on `shotAt`, falling back to `uploadedAt` where it is null.
- `filename-asc` / `filename-desc` on the filename, **case-insensitively**: `IMG_*.JPG` and `img_*.jpg`
  from two cameras in one album would otherwise separate into two blocks.
- **Every mode is a total order**, with ties broken on `photoId` — the rule the library read already uses.
  Without it, two recomputations of an unchanged album can differ, which would reshuffle unrelated
  photographs on an add and change the public page for no reason a curator could explain.
- `manual` returns the existing order untouched, so callers need no special case.

## Acceptance Criteria

- [ ] One pure function, table-driven tests per mode
- [ ] The `shotAt`-null fallback is covered, including a mix of null and set in one album
- [ ] Case-insensitive filename order is covered
- [ ] Sorting an already-sorted album is a no-op (the property an add depends on)
- [ ] Every photograph missing both keys still yields a stable order
- [ ] Items whose photograph is not in the library keep a defined position rather than vanishing

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R2/R4/R6.

## What changed

`sortAlbumOrder(items, mode) []string` in `adminalbum.go`, beside `moveAlbumOrder` and the same shape: a list
in, a list out, no database. `albumSortKey(mode)` says what a mode compares and **whether this binary can
compare it at all**.

The sort keys travel on `album.CuratorItem` (`SortShotAt`, `SortUploadedAt`, `SortFileName`) because the
album's item read already joins the photograph for its thumbnail. A second read keyed by id would be a second
place for "which photographs are in this album" to be decided, and the two disagreeing is how an album
silently loses a position.

**The `shotAt` fallback is applied in Go, not as a `COALESCE` in SQL.** Both are the projection's fixed-width
`YYYY-MM-DD HH:MM:SS`, so comparing them as strings compares them chronologically — but putting the rule in
SQL would make it a thing no test can exercise without a database. It is one table-driven case instead.

## Two things found by writing the tests

**1. An unrecognised mode reordered the album by content hash.** The first draft gave unknown modes an empty
key, so every photograph compared equal and the `photoId` tiebreak sorted the whole album by hash. `sortMode`
is a column, so a binary *will* meet a value a newer one wrote — a rollback would have silently shuffled
curated work. Now `albumSortKey` returns `(nil, false)` for anything it does not know, and that takes the same
path as `manual`: leave it exactly as it is. The only answer that cannot destroy work.

This is also why `ValidSortMode` and `albumSortKey` are deliberately **not** the same question. One is "may a
curator choose this", the other is "can I compute it"; a value already in the column has passed the first on
some earlier binary and still has to be survivable under the second.

**2. A test that could not fail.** `TestEverySortModeHasAKey` used two items and asserted the result was not
the input order — but for a *descending* mode the sorted answer and the input can legitimately be the same
list. Three items in an order that is neither ascending nor descending fixes it.

## Acceptance Criteria

- [x] One pure function, table-driven tests per mode
- [x] The `shotAt`-null fallback is covered, including a mix of null and set in one album
- [x] Case-insensitive filename order is covered
- [x] Sorting an already-sorted album is a no-op (the property an add depends on)
- [x] Every photograph missing both keys still yields a stable order
- [x] Items whose photograph is not in the library keep a defined position rather than vanishing

## Progress Log

- 2026-09-28 — Picked up. Modelled on `moveAlbumOrder`: the same signature shape, so the two read as one idea.
- 2026-09-28 — Chose to mirror the whole comparison for descending, tiebreak included, rather than "key desc,
  id asc". Both are total orders; this one has the property a curator would assume, that reversing the mode
  reverses the album.
- 2026-09-28 — ✅ Found and fixed the unknown-mode hash shuffle, and a test of mine that could not fail.
- 2026-09-28 — Recorded that filename order is by code point, so "æ" follows "z". A collation-aware order
  would mean the database doing the sorting, which §8 D1 rejected for stronger reasons.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean.
