# 374 — The contact sheet, its filters and its selection model

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

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

## What landed

`GET /api/admin/photos` and `GET /api/admin/photos/{photoId}/media` in `adminlibrary.go`, plus the contact
sheet, the nine filter presets, the selection model and the pinned action bar in `adminpage.go`.

**Filters compose rather than being one mode.** PRD 022 §6 lists six, and a single `filter=` parameter would
make them mutually exclusive — but the curator's real question is often a conjunction, and "in no album *and*
without a position" is precisely where the bulk-position workflow starts. So the query parameters compose and
the UI offers presets over them.

**An unrecognised filter value is refused, not ignored** — and this is the one that could have been a quiet
disaster. A typo'd `verdict=insid` silently ignored would show the curator *everything* while they believed they
were looking at the plottable subset, and the action bar acts on what is selected. The difference between
refusing and ignoring is the difference between a confusing screen and a bulk edit applied to the wrong forty
photographs. A malformed `limit`, by contrast, falls back — the asymmetry is deliberate: a page size is
cosmetic, a filter decides what an action applies to.

**The selection is held as a Set of ids, not read off the DOM.** A DOM-derived selection would be lost by any
re-render and would silently shrink to "what is currently loaded", which would make "select all matching this
filter" a lie the moment the grid was paged. It also survives filtering on purpose: a curator narrows, picks
forty, widens to check something — losing the forty would make the filters unusable as a working tool.

**"Select all matching the filter" pages the same read** rather than getting a bespoke endpoint. A dedicated
"all ids" route would be a second place the filter is interpreted, and the two disagreeing is exactly how a
bulk action lands on the wrong photographs.

## The media route is the dangerous one, and it needed a second attempt

A library photograph's id **is** its content ref, so handing the path segment to the blob store works perfectly
for every real photograph — and also serves every *other* object in the store to anyone holding the shared
password: a participant's portrait, a glimt somebody took down, a diploma. One password, one URL shape, the
whole store.

The first break-test **passed**, which was the useful part: changing *which* ref is used after the lookup proves
nothing, because for a real row the id and the ref are the same string. The dangerous edit is skipping the
lookup, and re-breaking it that way failed the test immediately. Worth recording, because the first version of
the test would have given false confidence.

Verified live with the strongest available evidence: the dev blob store holds **69 objects** against **2**
library rows. A library photograph's own ref serves; three real objects that are not library rows are refused.

## A wire-format bug the Go tests structurally could not catch

`adminCountsView` had no JSON tags, so the API exposed `Total` while the page's script read `total` — the header
silently stopped updating after a batch. Nothing failed: the template is indifferent to tags, and the Go tests
decode into the same struct, so the casing round-trips perfectly. Only looking at the live endpoint showed it.

Now tagged, and covered by two tests that close the loop the wire sits in the middle of: one asserts the wire
names against a loosely-decoded payload, the other that the page binds every one of them. The header refreshes
wholesale rather than just the total, because the numbers are read together and a stale second figure is worse
than none.

A related trap: `renderAdminPage` had no library stub, so the page rendered its *unavailable* branch and the
counts block was absent — making the header test fail for a reason unrelated to the header.

## Acceptance Criteria

- [x] The library lists newest-first, paged, with the header counts travelling on the same response so the grid
      and the header cannot disagree
- [x] All six filters work, compose, and are each one query — every one verified live
- [x] Click, shift-click range, ctrl/cmd-A over the loaded page, and "select all matching the filter" all work;
      the selection survives filter changes and paging
- [x] Out-of-bounds and unevaluated positions are marked *as such* on the thumbnail, and `unknown` is styled as
      a caveat rather than a rejection — it is a statement about us. `none` gets no badge, since having no
      coordinate is the ordinary case
- [x] The media handler resolves the id through the curator projection — tested with a valid ref that is not a
      row, verified by breaking it *correctly* on the second attempt, and confirmed live against 69 objects
- [x] Selection is keyboard-operable: the grid is a `listbox`, arrows move, space toggles, and every control is
      focus-visible
- [x] Both endpoints carry OpenAPI annotations with `@Failure 401`
- [x] Admin media is `no-store` — the bytes are immutable, but a contact sheet of the event's photographs in a
      shared laptop's disk cache outlives the session that fetched it

## Not done here

- **The four actions are present but disabled.** The bar and the selection they act on are this task's; the
      actions are tasks 375–379. They are rendered disabled rather than absent so the shape of the tool is
      visible and this task's selection can be exercised against the bar it will drive.
