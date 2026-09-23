# 366 — The curator's read interface, separate from the public one so a draft cannot leak

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

The curator needs the opposite of what the public gets: every album including unpublished and deleted
ones, every item including removed ones, every photograph including the ones in no album. Add
`album.CuratorQueries` and `photo.CuratorQueries` as **new interfaces**, wired onto their own fields on
`app.models` in `go/internal/data/models.go`.

**Do not widen `album.Queries`.** PRD 022 §8.4 and §8.8: that interface is publication-filtered in SQL,
it is read by unauthenticated handlers, and its doc comment calls that shape the privacy boundary.
`BySlug` deliberately returns the same "not found" for unknown, unpublished and deleted so drafts
cannot be enumerated. Adding a `includeUnpublished bool` parameter — the obvious cheap alternative —
would mean every public handler is one argument away from serving a draft, and the guarantee degrades
from a property of the type to a habit of the caller.

The reasoning to copy is the one `publicpatrol.table.go` already records: *"It is not here" is a
property; "we do not select it" is a habit.* A public handler holding only `app.models.Albums` is
**structurally unable** to reach a draft read, and that is worth a second interface and a little
duplication in the SQL.

The curator reads this task owes: the year's library paged newest-first with the filters PRD 022 §6
lists (*not in any album*, *without location*, *with location*, *out of bounds*, *tagged*,
*untagged*), a count set for the header, one photograph by id, every album including unpublished with
its item count, and one album's items in ordinal order including removed ones.

## What landed

`album.CuratorQueries` (`album/curator.go`) and `photo.CuratorQueries` (`photo/curator.go`), each on a
**distinct querier type** rather than extra methods on the existing one. That detail is the mechanism, not a
style choice: if the curator reads were methods on `querier`, then `photoQueriesOrNil` could hand a public
handler something that answers draft reads. Two types force the wiring to choose, and the choice is one
visible line in `main.go` — `data.WithCuratorReads(...)`, which is the entire answer to "what can see an
unpublished album?".

Reads delivered: the paged library with all six filters and a count set; one photograph including a deleted
one; a photograph's tags; every album including drafts and deleted; one album's items including removed ones;
`SlugTaken`; and `NextOrdinal`.

Three decisions worth recording:

**The curator's item read is a LEFT JOIN where the public one is INNER.** The public page wants a deleted
photograph to vanish; the curator needs to see that the position exists and that what was in it is gone.
`CuratorItem` therefore carries `Removed` *and* `PhotoDeleted` separately — which is exactly the distinction
PRD 022 §5 says the copy must make clear, since only one of the two is fixed by re-adding it here.

**`Counts` separates `WithLocation` from `Plottable`.** A curator who has placed forty coordinates and sees
"40 med position / 12 på kortet" has been told something true and actionable; one number would hide
twenty-eight rejections behind an encouraging total.

**`Tags` returns the id and number but not the patrol's name.** Resolving a number to a name already exists
in `publicpatrol.ByNumber`, and joining it here would mean two places decide what a patrol is called. The
handler resolves for display — which also keeps this out of `public_patrol` entirely.

**`NextOrdinal` counts removed rows.** Reusing a removed position would resurrect that row's soft delete
through the upsert, silently putting a taken-down photograph back on the page.

## The containment guard, and what it will catch

`cmd/api/curatorboundary_test.go` walks the package source and fails if any non-admin file reads
`models.AlbumCurator` or `models.PhotoCurator`. `adminOwnedFiles` is empty today — the admin handlers arrive
with tasks 372–379 — and the guard is deliberately in place *first*, so it fails the moment a curator read
is used from anywhere unlisted, including a file somebody meant to be admin-only but did not register.

It also asserts the complement, which is the more likely regression: that `album.Queries` has not grown an
`includeUnpublished`-style parameter. That is how the split would actually be defeated — it looks harmless in
a diff and turns the property into a habit. Verified by breaking it: adding a curator read to
`albumpage.go` fails the walk with instructions.

## Verified against the real database

None of this SQL is executed by any Go test — `cqrs.Reader` returns `*sql.Rows`, so a fake reader is not
expressible without a database or a mocking dependency, and conditional aggregates plus `NOT EXISTS`
subqueries are exactly where a silent semantic error hides. So the queries were run against the dev MariaDB
with seeded rows covering the interesting states:

- counts came back `total=2, inNoAlbum=1, withLocation=1, plottable=1, tagged=1, deleted=1` — all correct;
- the public item read dropped the position whose photograph was deleted, while the curator read showed it
  with `photoDeleted=1`;
- **the same photograph placed in both a published and a draft album plotted only from the published one**,
  which is the privacy property the whole interface split exists to protect, confirmed with data rather than
  by reading the WHERE clause.

Probe rows removed afterwards; the tables are back to empty.

## Acceptance Criteria

- [x] `album.CuratorQueries` and `photo.CuratorQueries` exist as separate interfaces with their own
      fields on `app.models`
- [x] `album.Queries` gains no method, no parameter and no visibility flag — asserted by a test that also
      pins the method count
- [x] Every filter and count PRD 022 §6 requires is answerable in one query each, paged
- [x] Both interfaces are nil-safe in the same way the existing ones are, via `albumCuratorOrNil` /
      `photoCuratorOrNil`, so a missing projection is a `503` rather than a panic
- [x] A test asserts no handler outside the admin surface holds a `CuratorQueries` value — verified by
      breaking it
- [x] Doc comments record why the split exists, referencing PRD 022 §8.8
- [x] The curator read is year-scoped and cannot return another year's rows — the year is the first
      condition of every statement, applied in `Filter.where` rather than per read
