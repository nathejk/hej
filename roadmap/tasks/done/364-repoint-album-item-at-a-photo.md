# 364 — Repoint `album_item` at a photo, and tolerate the legacy item event on replay

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-22
**Completed:** 2026-09-23

## Description

`album_item` becomes pure **membership**: which photo sits at which position in which album. Per
PRD 022 §8.3 it keeps `(albumId, ordinal)` as its primary key — which is what preserves the public
media URL `/api/public/albums/{albumId}/media/{ordinal}` unchanged — and replaces every media column
(`blobRef`, `thumbRef`, `caption`, `width`, `height`, `bytes`, `latitude`, `longitude`,
`boundsVerdict`) with a single `photoId`. Touches `go/nathejk/table/album/table.sql`, `events.go`,
`consumer.go` and `querier.go`.

`album.ItemAdded` correspondingly carries a `photoId` instead of the media fields. This is a breaking
change to a published event shape, and PRD 022 §8.7 gives the only honest reason it is acceptable:
**no album has ever been created outside the development fixture** (`go/cmd/api/devalbum.go`), because
there is no curation tool — which is why PRD 022 exists at all. So there is no production history to
respect, only dev history.

That dev history still has to be survived. The fold must **tolerate** a legacy `itemadded` — one with
a `ref` and no `photoId` — rather than error it. An error is logged and dropped by the stream library,
so a strict fold would turn every old dev fixture into a wall of warnings on every boot, and the
person reading those warnings would learn nothing from them. Recommended handling per §8.7: ignore
legacy item events and record the reason in the fold's comment. `itemremoved` is unchanged — it is
keyed `(albumId, ordinal)` and still fits.

**Hazard, called out because PRD §8.11 says it must be handled deliberately and not discovered:**
`CREATE TABLE IF NOT EXISTS` will **not** alter an existing `album_item`. A deploy onto a database
that already has the old table will silently keep the old columns and the replay will write nothing
useful. Dropping and rebuilding that table on boot is the intended path and needs to be explicit in
the code and in the deploy note.

## What landed, and the one thing that grew

`album_item` is now `(albumId, ordinal) → photoId`, the querier assembles `album.Item` by joining `photo`,
and the fold skips legacy events. The public media URL is unchanged, as required.

**This task had to absorb task 368 (the shared-blob check), and that was not a choice.** The two could not
be separated across commits without shipping a data-loss bug in between.

`album.RefsInUse` answered "does any live album item still reference these bytes?", and the glimt takedown
path united it with glimt's own answer before deleting an object. Once the refs moved to `photo`, the
obvious move was to reimplement it here as a join through `album_item` — and that would have been worse
than deleting it. A join answers the **narrower** question "which refs are used by photographs that are in
an album", and narrower is the dangerous direction for a purge, because the caller deletes whatever comes
back unused. A photographer's upload that no curator had arranged yet would have been reported unused and
had its bytes deleted by an unrelated glimt takedown.

So `RefsInUse` was removed from `album.Queries` outright, `glimtdelete.go` now asks
`photo.Queries.RefsInUse`, and `albumblobs_test.go` stubs the library instead of the albums — with a new
regression test, `TestGlimtDeleteKeepsBytesOfAPhotographInNoAlbum`, for exactly the case a join would have
missed. Verified by breaking it: disabling the library branch fails four tests.

**A second semantic consequence, also unavoidable here.** Removing an item from an album no longer purges
bytes at all, and neither does deleting an album. The photograph outlives both, so purging would blank it
in the library and in every other album — the old sharing check's bug, reintroduced from the other end.
Five tests asserting the old purge were replaced with two asserting the new invariant. Byte deletion now
belongs solely to the library's delete path (task 379).

## Two things the rebuild taught us

The hazard was real and the first attempt did **not** work. `photo` and `photo_patrol` were created while
`album_item` silently kept all nine old columns, which is precisely the "looks healthy, writes nothing"
failure PRD 022 §8.11 warned about — caught only because it was checked against the live dev database
rather than reasoned about.

The cause was that the dev container's gates pipeline had failed on a `staticcheck` U1000 (an unused test
helper orphaned by the moved coordinate tests), so the app never booted. Worth recording because the
symptom — new tables present, changed table untouched — looks exactly like a broken migration guard.

The guard itself deliberately **does not** follow `person/table.go`'s "additive only, never DROP" rule, and
the reasoning is in `dropLegacyAlbumItem`: that rule protects columns holding data with no other source,
while `album_item` is pure projection rebuilt from a stream that is never rewritten, and this DROP is
keyed on the old shape so it runs **once** rather than on every boot.

## Acceptance Criteria

- [x] `album_item` carries `photoId` and no media, caption or coordinate columns
- [x] `(albumId, ordinal)` is still the primary key, and the public media URL shape is unchanged
- [x] `album.ItemAdded` carries `photoId`; the fold validates it as a content hash
- [x] A legacy `itemadded` (a `ref`, no `photoId`) is ignored with the reason recorded in the fold and does
      **not** fail the replay — covered by `TestItemAddedSkipsALegacyEvent`, verified by breaking it
- [x] The rebuild of the changed table is deliberate and documented, not left to
      `CREATE TABLE IF NOT EXISTS`; **verified against the live dev database holding the old shape**, and
      verified once-only by probing a row through a second boot
- [x] Adding a photo already in an album is a no-op rather than a duplicate row — `UNIQUE KEY album_photo`
- [x] The album page read still returns items in curator order after the change
- [x] Every join to `photo` filters `deleted = 0`, so a deleted photograph leaves the page, the cover, the
      item count and the map at once — which is why `photo`'s delete fold leaves `album_item` alone
- [x] `gofmt`, `go vet`, `staticcheck` and the full `go test ./...` pass
