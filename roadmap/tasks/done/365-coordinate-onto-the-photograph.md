# 365 — Move the coordinate and its verdict onto the photograph, where a photograph's facts belong

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

`latitude`, `longitude` and `boundsVerdict` move from `album_item` to `photo`, and the public map read
(`Plottable`) becomes a join `photo` → `album_item` → `album` answering "a published album's plottable
photographs". The index that made that read cheap on `album_item` (`year_plottable`) moves to `photo`.
Touches `go/nathejk/table/album/table.sql`, `querier.go`, and the new `photo` package from task 363.

The reason is PRD 022 §8.3: a location is plainly a fact about the *photograph*, and the brief asks
for "a location on one or more photos". With the coordinate on the membership row, the same photograph
in "Natten" and in "Målet" has two coordinates that can disagree, and the curator has no way to see
which one the map used.

Three things must not change, per PRD 022 §8.4:

- **The four verdicts stay four.** `none`, `inside`, `outside`, `unknown`, and `unknown` is not folded
  into `outside` — `outside` is a statement about the photograph, `unknown` is a statement about us.
  `album.Plottable` remains the single place the rule "only `inside` is plotted" lives.
- **`imaging.Prepare` still destroys EXIF by re-encoding.** The coordinate is read from the original
  bytes *before* that, by `imaging.ReadGPS`, exactly as `storeAlbumImage` does today. Both
  `album/table.sql` and `go/cmd/api/albummedia.go` carry a warning not to "fix" the pipeline because a
  feature wants a coordinate; this PRD adds a reason to want one and the warning still holds. Carry the
  warning across to `photo/table.sql`.
- **Setting a location re-runs the bounds check** and stores the new verdict (PRD 022 §6). A
  curator-placed point is not exempt — nothing reaches the public map unverified.

## What was already true, and what this task actually added

The column move and the joined `Plottable` read landed structurally with tasks 363 and 364 — they could
not be separated from them, since a table cannot lose a column in one commit and have the read repointed
in another. So this task was mostly about the parts that were *asserted* rather than built:

**The ingest now speaks the canonical vocabulary.** `albumMediaPrepared` carried a `Lat`, an `Lng` and a
`BoundsVerdict` side by side; it now carries a single `*photo.Location`. That is the same argument the event
shapes make — a coordinate and the judgement made about it are one fact, and three separate fields make
"moved the point, forgot the verdict" expressible. `albumBoundsVerdict` returns `photo.Bounds*` rather than
`album.Bounds*`, so the ingest no longer speaks the legacy names at all.

**A new source-level guard, `querysafety_test.go`.** The safety property of `Plottable` is that its
conditions are *in the SQL* rather than in a caller — that is the whole reason a handler "cannot plot an
unchecked coordinate by forgetting a condition". So the statement is what needed a test, and it had none.

The awkward part, recorded honestly in the file: it cannot be tested by execution. `cqrs.Reader` returns
`*sql.Rows`, which nothing outside `database/sql` can construct, so a fake reader is inexpressible without
a real database in the suite or a mocking dependency — and the `cmd/api` stubs bypass this file entirely,
which is precisely why a bug here would not surface there. Reading the source is the technique
`glimtopenapi_test.go` and `publicprivacy_test.go` already use for invariants the type system cannot hold.
It checks the presence of a clause, not the behaviour of a database, and says so.

Four guards: every condition `Plottable` must filter on (each with the consequence of its absence spelled
out in the failure message), that no read takes a photograph's facts from the membership row, that every
join to `photo` requires it live, and that the count and cover subqueries see the same photographs as the
page. Verified by breaking it: dropping `p.deleted = 0` fails two of them.

## One criterion belongs elsewhere

"Setting a location re-runs the bounds check" is about the curator's bulk-position action, which does not
exist yet. The projection half is in place — `photo.Updated` cannot carry a coordinate without a verdict,
by construction — but the handler that calls `albumBoundsVerdict` on a curator-placed point is task 376,
which owns that criterion.

## Acceptance Criteria

- [x] `photo` owns `latitude`, `longitude` and `boundsVerdict`; `album_item` owns none of them — asserted
      by `TestNoReadTakesACoordinateFromTheMembership`
- [x] The plottable read joins photo → item → album and returns only `inside`, only from published,
      non-deleted albums and non-deleted items and photos
- [x] A test proves no coordinate with a verdict other than `inside` can reach the public map —
      `TestPlottableFiltersEverythingItMust`, which also refuses to let the query mention any other verdict
- [x] `unknown` is still distinguishable from `outside` end to end, including through the ingest
- [x] The EXIF warning is carried into `photo/table.sql`, and `ReadGPS` still runs before `imaging.Prepare`
      — still guarded by `TestStoreAlbumImageReadsGPSThenDestroysIt`, which reads the stored bytes back
- [x] The `year_plottable` index exists on `photo`, keyed `(year, deleted, boundsVerdict)`
- [x] One photograph in two albums has exactly one coordinate and one verdict — structural, since both
      memberships resolve the same `photo` row
- [ ] ~~Setting a location re-runs the bounds check~~ — moved to task 376, which owns the handler
