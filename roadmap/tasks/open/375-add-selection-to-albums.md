# 375 — Put a selection in one or more albums

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

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

## Acceptance Criteria

- [ ] One request adds N photographs to M albums, and the response reports what was added and what was
      already there
- [ ] Re-adding an existing member is a no-op, with no duplicate row and no ordinal churn
- [ ] Creating an album inline works and the album is created **unpublished**
- [ ] A photograph in two albums has one coordinate, one caption and one tag set — tested
- [ ] Removing from one album leaves the photograph in the other and in the library
- [ ] Ordinal assignment under two concurrent adds is defined and tested, not accidental
- [ ] A broken stream yields `503` and the selection is not shown as filed
- [ ] OpenAPI annotations with `@Failure 401`
