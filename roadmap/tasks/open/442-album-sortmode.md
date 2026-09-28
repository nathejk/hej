# 442 — album.sortMode

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 024 §6 R1/R9. The album gains a sort mode: `manual`, `time-asc`, `time-desc`, `filename-asc`,
`filename-desc`.

**The default lives in two places, deliberately** (PRD 024 §6 R9):

- the **column** defaults to `manual`, the value that means "never recompute";
- the **create handler** sets `time-asc` on the `Created` event.

So a new album is time-ordered because code chose that, and every album that exists today keeps its
arrangement because the column's default is the inert value. Defaulting the column to `time-asc` would
have re-sorted every existing album the first time anybody added a photograph to it — and, since there is
no backfill, re-sorted it on the `uploadedAt` fallback, i.e. shuffled a hand-arranged album into roughly
upload order. No migration, and nobody has to think about old albums.

This task is data and reads only: no sorting, no interaction. The value list belongs in the column comment
as `boundsVerdict` does, and in one place in Go that the handlers validate against.

## Acceptance Criteria

- [ ] `album.sortMode VARCHAR(16) NOT NULL DEFAULT 'manual'`, values documented in the column comment
- [ ] `Created.SortMode` and `Updated.SortMode *string`; fold to match
- [ ] One Go definition of the valid values, with a validator the handlers use
- [ ] `CuratorAlbum` carries it, so the editor card can render it
- [ ] A new album is created `time-asc`; an album folded from a log with no `sortMode` is `manual`
- [ ] No public read gains the field (it is a curator's setting, not a property of the published album)

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R1/R9.
