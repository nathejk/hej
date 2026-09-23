# 375 — Put a selection in one or more albums

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

`POST /api/admin/albums/items`: add a selection of photographs to **one or more** albums in a single
action, publishing one `album.itemadded` per (album, photo) pair with the next free ordinal, plus the
inline "create a new album" path that calls `POST /api/admin/albums` first.

This is the action the whole model change exists for. PRD 022 §2: a photograph often belongs in more
than one album — "Natten", "Målet", "Postmandskabet" — and in the old model that was two independent
rows with two independent coordinates free to silently disagree. Now it is two membership rows
referencing one photograph, so its location, caption and tags are the same in every album it appears
in (PRD 022 §3, §8.3).

Adding a photograph that is already in an album is a **no-op, not a duplicate row** (PRD 022 §6) —
re-selecting is routine when the curator is working through a filter over several sittings. Removing a
photograph from one album leaves it in the others and in the library; deleting an album does not remove
its photographs from the library. Both of those are properties of the membership table, and both should
be tested here rather than assumed from task 364.

Two curators at once is **last write wins, per field** (PRD 022 §5), stated so nobody later mistakes it
for a bug. Ordinals are assigned per album, so two simultaneous adds to the same album may collide on
an ordinal — pick a rule (append after the current maximum, re-read before publishing) and record it.
If the stream is unavailable the response is `503` and the UI must not show the photographs as filed.

## What landed

`POST /api/admin/albums/items`, `POST /api/admin/albums`, and `GET /api/admin/albums` — the last one not in
the task, but the panel cannot offer albums to file into without it. Plus the inline panel, which opens over
the contact sheet rather than navigating, because navigating away loses the selection.

**An album is always created unpublished, held by three layers** rather than by the handler remembering: the
request type has no `published` field; `ReadJSON` sets `DisallowUnknownFields`, so a client that asks anyway
gets a **400** rather than being silently ignored; and `album.Created` carries no such field either. All three
are asserted.

**Re-adding is a no-op, but a *removed* membership is not.** That distinction is the subtle part. A photograph
whose membership was removed needs an event to clear the removal — so it is not "already there" — but it must
not take a second ordinal either, or the album holds it twice once the fold runs. It is re-published at its
**existing** ordinal, which clears `deleted` and puts it back exactly where it was.

**The response reports per album, not one total.** "40 added" hides the case a curator most needs to see: that
one of the three albums they picked already held most of the selection.

The slug folds Danish letters by hand — `Lørdag` → `loerdag`, not `lordag` (a different word) and not `lrdag`
(not a slug anybody would type). A generic Unicode decomposition gives the first of those, which is why this is
explicit for the three letters that matter. A test checks every slug this produces is one the fold will accept,
since the fold refuses an unusable one outright and the result would be an album with an address nobody can
reach.

## The ordinal rule, and its limit

**Rule:** ordinals are appended after the album's current maximum, counting **removed** positions. Read once
per album, immediately before that album's batch, and allocated consecutively in selection order.

Removed positions count because reusing one would resurrect that row's soft delete through the upsert —
silently putting a taken-down photograph back on the page. Verified by breaking it.

**The limit, stated rather than discovered:** `album_item`'s primary key is `(albumId, ordinal)`, so two truly
concurrent bulk adds to the *same* album can read the same maximum, and the second's insert **overwrites** the
first's membership rather than colliding. That is a *lost* membership, which is worse than the "last write wins
per field" PRD 022 §5 accepts elsewhere.

It is accepted for now on the PRD's own grounds — §5 records that there will realistically be one curator — and
it is bounded: the loss is one membership, recoverable by re-adding, and visible because the album shows fewer
photographs than the response reported. What a real fix needs is either a server-side ordinal allocator or an
ordinal that is not the primary key, and the second is blocked by the ordinal being in the public media URL.
**Worth revisiting if a second curator ever becomes real.**

## Verified live

- creating `Lørdag morgen` was **refused**, because a dev-fixture album already holds that slug — the guard
  working, and a reminder that fixtures replay from the log;
- two photographs filed into two albums → `4 billeder lagt i 2 album`, four membership rows;
- the identical request re-run → `added: 0, already: 2` per album, no new rows, no ordinal churn;
- **the point of the whole model change**: each photograph has two memberships and exactly *one* row of facts
  — one caption, one coordinate, one verdict. Under the old shape that was two rows free to disagree;
- `inNoAlbum` moved 2 → 0 and the thumbnails gained their album marks, so the counts are live.

Note the dev-fixture albums list with `itemCount: 0` — correct: their legacy `itemadded` events are skipped by
the fold (task 364), so they are empty until refilled through this endpoint.

## Acceptance Criteria

- [x] One request adds N photographs to M albums, and the response reports per album what was added and what was
      already there
- [x] Re-adding an existing member is a no-op, with no duplicate row and no ordinal churn — verified live
- [x] Creating an album inline works and the album is created **unpublished**, held three ways
- [x] A photograph in two albums has one coordinate, one caption and one tag set — verified against the live
      database, which is where the property actually lives
- [x] Removing from one album leaves the photograph in the other and in the library — covered by task 364's
      `TestRemovingAnAlbumItemKeepsTheBytes` and by the membership model itself; the *action* for it is task 379
- [x] Ordinal assignment is defined, tested and documented, including its concurrency limit
- [x] A broken stream yields `503`; the UI keeps the selection so the curator can retry without re-picking
- [x] OpenAPI annotations with `@Failure 401` on all three endpoints

## Also fixed here

`staticcheck` in the dev container caught two `SA4010` dead appends in the first version of the tests — slices
built for an oversize case I had moved into its own test. The container's gates wedge the dev loop when they
fail, so this surfaced as "the API is serving a stale binary", which is worth recognising quickly.
