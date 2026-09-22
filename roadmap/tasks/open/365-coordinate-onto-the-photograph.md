# 365 — Move the coordinate and its verdict onto the photograph, where a photograph's facts belong

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

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

## Acceptance Criteria

- [ ] `photo` owns `latitude`, `longitude` and `boundsVerdict`; `album_item` owns none of them
- [ ] The plottable read joins photo → item → album and returns only `inside`, only from published,
      non-deleted albums and non-deleted items and photos
- [ ] A test proves no coordinate with a verdict other than `inside` can reach the public map
- [ ] `unknown` is still distinguishable from `outside` end to end, including in the curator read
- [ ] The EXIF warning is carried into `photo/table.sql` verbatim in spirit, with the coordinate read
      still happening before `imaging.Prepare`
- [ ] The `year_plottable`-equivalent index exists on `photo` and the map read does not scan the year
- [ ] One photograph in two albums has exactly one coordinate and one verdict
