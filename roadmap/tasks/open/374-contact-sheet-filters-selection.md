# 374 — The contact sheet, its filters and its selection model

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

`GET /api/admin/photos` (paged, filtered) and `GET /api/admin/photos/:photoId/media`, plus the contact
sheet that renders them: thumbnails newest first, a filter bar, and a selection action bar pinned to
the bottom when anything is selected — *Tilføj til album · Sæt position · Tag patrulje · Slet*.

The filters PRD 022 §6 requires: *not in any album*, *without location*, *with location*, *out of
bounds*, *tagged with a patrulje*, *untagged*. Selection supports click, shift-click for a range, and
**"select everything matching this filter"** — because "these forty are from Post 3" is the true shape
of the work (PRD 022 §3) and a selection model that tops out at what fits on screen would push the
curator back to doing it forty times, which is how it does not get done.

Each action opens a **small inline panel rather than navigating away**, because navigating away loses
the selection (PRD 022 §7). That is the one hard constraint on the layout.

The media route must follow the existing rule from PRD 022 §8.4: **blobs are never addressed by ref in
a URL**. Always `(entity, ordinal[, variant])` → a projection read that applies the visibility filter →
`blob.Ref` → `streamGlimtMedia`. Here the entity is the photo id, which *is* a content ref — so the
handler must still go through the curator projection read rather than handing the path segment to the
blob store, or the route becomes a general content-addressed file server behind one shared password.

Reads go through `photo.CuratorQueries` (task 366), never `album.Queries`, so draft and deleted rows are
reachable here and structurally unreachable from anything public.

## Acceptance Criteria

- [ ] The library lists newest-first, paged, with the header counts ("312 billeder · 47 uden album ·
      12 med position")
- [ ] All six filters work and compose sensibly; each is one query
- [ ] Click, shift-click range and "select all in filter" all work, and the selection survives every
      action panel opening and closing
- [ ] Out-of-bounds and unevaluated positions are shown *as such* on the thumbnail, with the copy
      PRD 022 §7 asks to be written carefully
- [ ] The media handler resolves the id through the curator projection, not by handing the path to the
      blob store — tested with an id that is a valid ref but not a row
- [ ] Selection is keyboard-operable and controls are focus-visible
- [ ] Both endpoints carry OpenAPI annotations with `@Failure 401`
