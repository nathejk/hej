# 364 — Repoint `album_item` at a photo, and tolerate the legacy item event on replay

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

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

## Acceptance Criteria

- [ ] `album_item` carries `photoId` and no media, caption or coordinate columns
- [ ] `(albumId, ordinal)` is still the primary key, and the public media URL shape is unchanged
- [ ] `album.ItemAdded` carries `photoId`; the fold validates it
- [ ] A legacy `itemadded` (a `ref`, no `photoId`) is ignored with a logged reason and does **not**
      fail the replay — covered by a test in `album/consumer_test.go`
- [ ] The rebuild of the changed table is deliberate and documented, not left to
      `CREATE TABLE IF NOT EXISTS`; verified against a database holding the old shape
- [ ] Adding a photo already in an album is a no-op rather than a duplicate row
- [ ] The album page read still returns items in curator order after the change
