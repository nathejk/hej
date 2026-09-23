# 378 — The album editor: fields, order, captions, and the publish switch

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

`GET /admin/album/:slug` (HTML), `GET /api/admin/albums`, `POST /api/admin/albums`,
`PATCH /api/admin/albums/:albumId` and `PATCH /api/admin/albums/:albumId/items`: an album's own page
with its title, description, sort order, publish toggle, and its items in order with caption fields and
an ordinal.

**Create is always unpublished** (PRD 022 §6). Not a default the curator can override at creation: an
album is assembled over several sittings, and a create that could publish would put the first
photograph on the open web before the second was chosen. Publishing is a `PATCH`, expressed by the
absence of `Published` on `album.Created` — the reasoning `album/events.go` already records.

**The slug is frozen at creation.** Retitling must never break a link already shared, which on a public
page is a link in a family's chat history. The existing `Updated` event deliberately has no slug field;
keep it that way.

The editor reads through `album.CuratorQueries` (task 366) so it can see unpublished and deleted albums
and removed items. No public read may gain that ability — `album.Queries` stays publication-filtered in
SQL, and `BySlug` keeps returning the same "not found" for unknown, unpublished and deleted so drafts
cannot be enumerated (PRD 022 §8.8). Note the admin page is addressed by **slug**, like the public page,
so a draft's slug is guessable from the public 404 only if the public read leaks — hence the test.

Captions live on the **photo**, not the item (PRD 022 §8.3): one place to edit, shared by every album it
is in. A per-album caption override is imaginable and is deliberately deferred (§11 Q3) rather than
built speculatively. The **cover is the first live item** — no cover column. Ordinals are editable
numbers; drag-to-reorder is explicitly not a launch requirement (PRD 022 §4).

Unpublishing or deleting must drop the album from the frontpage within the page's 60-second cache
window and no longer (PRD 022 §6) — "we will take it down" has to be true.

## What landed

`GET /admin/album/:slug`, `PATCH /api/admin/albums/:albumId`, `PATCH /api/admin/albums/:albumId/items`, and the
caption path on `PATCH /api/admin/photos`. (`GET`/`POST /api/admin/albums` landed with task 375, which needed them
to file anything.)

The editor is addressed by **slug**, like the public page, so a curator checking their work can move between
`/2026/album/natten` and `/admin/album/natten` by editing the URL. That is safe because the public read does not
leak: `album.Queries.BySlug` answers an identical "not found" for unknown, unpublished and deleted, so a draft's
slug is not discoverable from the open web.

## The reorder needed its own event, and the schema forced it

This was the one genuinely hard part, and it was not visible from the task.

`album_item` has `PRIMARY KEY (albumId, ordinal)` **and** `UNIQUE (albumId, photoId)`. Moving an item into a
position another holds collides on whichever key the write touches first, and MariaDB has no deferred constraint
check — so **a plain swap is not expressible as two independent writes at all.** There is no ordering of
per-item updates that avoids it.

So reordering is one event carrying the album's whole new order (`album.ItemsReordered`), and the fold applies it
in two passes: offset every ordinal in the album to vacate the range, then place each photograph at its index.
Three details matter and are all tested:

- **the offset covers removed rows too** — a soft-deleted row still occupies its ordinal *and* its photo id, so
  leaving it behind would put it in the way of a live item;
- **placement matches on photo id, not ordinal** — which is what makes the fold idempotent on replay, since after
  the first pass the offset has already moved;
- the event **names photographs rather than ordinals**, so it cannot express a contradiction like two items at
  position 3, and reads a year later as "this album's order is now this".

Verified where it counts: **a live swap against the real database**, which is precisely the case that collides
without the offset. It went through with no duplicate-key error and nothing in the dead-letter queue. Verified by
breaking it too — removing the offset statement fails the test.

## Three layers keep the slug frozen

A retitled album answering 404 is a dead link in a family's chat history, so: no `slug` field on the request, no
`slug` field on `album.Updated`, and `ReadJSON`'s `DisallowUnknownFields` turns an attempt into a **400** rather
than a silent no-op. Confirmed live — `{"slug":"noget-andet"}` → `json: unknown field "slug"`.

The page shows the slug read-only *and says why*, because a field a curator cannot change reads as a bug unless
explained.

## Captions live on the photograph

One caption, written once, read by every album. Verified live: a photograph in two albums, caption set once, and
both albums read the same prose from one row. Under the pre-PRD-022 shape that was two rows free to drift — which
is the divergence the library split removed, so a per-album override (§11 Q3) stays deferred rather than being
built speculatively.

Saved on blur rather than with a button, because a caption per photograph would otherwise mean a button per
photograph. The page states that the caption is shared, above the list, since it would otherwise be a surprise.

## Two smaller decisions

**Publication is sent alone.** The publish buttons send only `{published}`, so an unsaved half-typed title in the
form above cannot ride along with it. A curator pressing *udgiv* has said one thing.

**Reordering moves rows in the DOM and saves on demand**, so a curator can shuffle freely and change their mind
without a dozen events on a log that is never rewritten. Arrow buttons rather than drag handles — PRD 022 §4 puts
drag-to-reorder outside the launch requirements, and two buttons are keyboard-operable for free.

## Acceptance Criteria

- [x] A created album is unpublished, and there is no request shape that creates a published one (task 375, held
      three ways)
- [x] The slug is set at creation and no endpoint can change it — three layers, confirmed live
- [x] Title, description, sort order and published are editable, and pointer semantics mean editing one does not
      wipe another — verified live: changing only the title preserved the description, the sort order and the
      publication
- [x] Editing a caption changes it in every album the photograph is in — verified live across two albums
- [x] Reordering works and the cover follows the first live item — the cover is computed as the first live,
      undeleted item, since there is no cover column
- [x] Unpublishing removes the album from the frontpage within 60 seconds — the frontpage's cache window is
      asserted at `max-age=60` by task 335's `TestRemovalPromptnessIsBoundedByThePageCache`, and unpublishing takes
      effect on the next read
- [x] The public `BySlug` still answers an identical "not found" for unknown, unpublished and deleted — unchanged,
      and guarded by `TestThePublicAlbumInterfaceHasNoDraftSwitch` from task 366
- [x] Every endpoint carries OpenAPI annotations with `@Failure 401`

## A test-maintenance note

Inserting the caption handler between two existing functions broke `TestUpdatedCannotExpressAClear`, which sliced
the source file from one function to the *next comment heading* — so it started reading the caption code, which
legitimately publishes `photo.Updated`. Now sliced to the function's own body. Worth recording because
source-reading guards are load-bearing here and this is how they rot.
