# 442 — album.sortMode

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

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

- [x] `album.sortMode VARCHAR(16) NOT NULL DEFAULT 'manual'`, values documented in the column comment
- [x] `Created.SortMode` and `Updated.SortMode *string`; fold to match
- [x] One Go definition of the valid values, with a validator the handlers use
- [x] `CuratorAlbum` carries it, so the editor card can render it
- [x] A new album is created `time-asc`; an album folded from a log with no `sortMode` is `manual`
- [x] No public read gains the field (it is a curator's setting, not a property of the published album)

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R1/R9.
- 2026-09-28 — Implemented, data and reads only. Schema: `album.sortMode VARCHAR(16) NOT NULL DEFAULT
  "manual"`, placed next to `sortOrder` with a comment saying explicitly that the two are unrelated (albums
  on the frontpage vs. photographs within an album), and recording R9's reasoning for why the column's
  default is the inert value.
- 2026-09-28 — Events: `Created.SortMode string` (empty = say nothing), `Updated.SortMode *string`. The
  create fold sets the column **on insert only** and leaves `sortMode` out of the `ON DUPLICATE KEY UPDATE`
  clause, the same rule `published`/`deleted` follow — otherwise a re-delivered create would re-sort an album
  a curator had switched to `manual`, which is the same irreversible loss R9 is guarding against, reached by
  replay rather than by default. Not stated in the task or the PRD; recorded in the fold's comment.
- 2026-09-28 — `album.SortMode*` constants, `SortModes()` and `ValidSortMode` in `events.go` beside the bounds
  verdicts, which is where this package already keeps "constants plus the one function that decides". `""` is
  **not** valid: "not mentioned" is a nil pointer, so a blank mode is a malformed message rather than a
  synonym for `manual`. Both folds refuse an unknown value.
- 2026-09-28 — `CuratorAlbum.SortMode`, selected and scanned in both curator reads. The public `Album` and
  `Item` are untouched, as are `Published`, `BySlug` and `Plottable`.
- 2026-09-28 — `createAdminAlbum` sets `time-asc` unconditionally, with no parameter, for the same reason
  there is none for `published`. `PATCH /api/admin/albums/{albumId}` accepts `sortMode` and answers 400 with
  a Danish message listing the accepted values; it does **not** recompute any order — that is task 443.
  `adminAlbumSummary` gained `sortMode` so the editor card has something to render (PRD 024 §8). OpenAPI
  annotations on the list, create and update endpoints updated.
- 2026-09-28 — Tests: `go/nathejk/table/album/sortmode_test.go` and
  `go/cmd/api/adminalbumsortmode_test.go`. Each behavioural assertion was checked to fail before it passes by
  mutating the source (dropping the create handler's mode, dropping the handler's validation, dropping the
  response/list mapping, writing `manual` explicitly when the event says nothing, adding `sortMode` to the
  upsert's update clause, dropping the fold's validation, making `""` valid). Two guards pass trivially and
  are guards rather than regressions: "a create must not accept a client-chosen mode" (held by
  `DisallowUnknownFields`, like `published`) and "an update that does not mention the mode must not write
  it".
- 2026-09-28 — `gofmt -l .` silent; `go vet ./...` and `go tool staticcheck ./...` clean; `GOWORK=off go test
  ./...` green.
