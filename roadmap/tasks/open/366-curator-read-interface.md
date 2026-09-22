# 366 — The curator's read interface, separate from the public one so a draft cannot leak

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

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

## Acceptance Criteria

- [ ] `album.CuratorQueries` and `photo.CuratorQueries` exist as separate interfaces with their own
      fields on `app.models`
- [ ] `album.Queries` gains no method, no parameter and no visibility flag
- [ ] Every filter and count PRD 022 §6 requires is answerable in one query each, paged
- [ ] Both interfaces are nil-safe in the same way the existing ones are, so a missing projection is a
      `503` rather than a panic
- [ ] A test asserts no handler outside `/api/admin` or `/admin` holds a `CuratorQueries` value
- [ ] Doc comments record why the split exists, referencing PRD 022 §8.8
- [ ] The curator read is year-scoped and cannot return another year's rows
