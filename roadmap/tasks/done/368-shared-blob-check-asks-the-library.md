# 368 — The shared-blob check asks the library, and still refuses to guess

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-22
**Completed:** 2026-09-23

## Description

`RefsInUse` moves from `album.Queries` to the library: the question "does anything still reference
these bytes" is now answered by `photo`, because that is where `blobRef` and `thumbRef` live after
task 364. Touches `go/nathejk/table/album/querier.go`, the new `photo` querier,
`go/internal/data/models.go`, `go/cmd/api/albumremove.go` (`albumRefsUsedElsewhere`) and
`go/cmd/api/glimtdelete.go`, which depends on it.

Two existing rules must survive intact, both recorded in `models.go` and `albumremove.go`:

- **Any error means nothing is deleted.** `RefsInUse` returning an error is not "no refs in use"; the
  delete path aborts. Content addressing means a blob may be shared with a glimt — an organizer
  curating a photograph a participant also posted publicly is the expected workflow, not a corner
  case — so a wrong answer here deletes someone else's picture.
- **The exclusion is not optional.** `RefsInUse` is a year-wide question, and because the fold is
  asynchronous the rows being removed are still live when the check runs. Without naming them the
  check reports their own refs as in use, looks entirely correct, and deletes nothing ever. A test
  already caught that once; it must keep catching it. The exclusion key changes shape from
  `album.ItemKey` to a photo id, and that is the part to get right.

PRD 022 §8.9 adds one more requirement: there are now **two doors to the same removal** — the
curator's `DELETE /api/admin/photos/:photoId` and the existing organizer-in-the-app endpoints, which
stay exactly as they are because they answer a different question ("somebody complained and I am at a
barbecue with my phone"). Two doors is correct; the two disagreeing about blob purging is not. Both
must go through the **same purge helper**.

## Done with task 364, because it could not safely be deferred

The two tasks were written as a sequence, and that sequencing was wrong: splitting them across commits
would have left a commit in which a glimt takedown deletes a photographer's bytes.

The trap was that `album.RefsInUse` looks *subsumed* after the refs move — reimplementable as a join
through `album_item`. It is not. The join answers the narrower question "which refs are used by
photographs that are **in an album**", and narrower is the dangerous direction for a purge because the
caller deletes what comes back unused. An upload no curator has arranged yet would be reported unused.
So the method was deleted rather than moved, and a note where it used to live in `album/querier.go`
records why, so nobody helpfully restores it.

See task 364 for the full account. What landed:

- `photo.Queries.RefsInUse(year, excluding []string, refs []string)` — exclusion keyed on photo ids.
- `album.Queries.RefsInUse` and `album.ItemKey` removed.
- `glimtdelete.go`'s `refsUsedElsewhere` asks glimt and the library, unions them, and still fails as a
  whole rather than proceeding on a partial answer.
- `albumblobs_test.go` stubs the library, plus a new regression test for the in-no-album case.

## Two criteria that changed meaning rather than being met

**The exclusion regression test did not survive, and should not have.** It guarded the album *removal*
path, which no longer purges anything: after the split, taking a photograph out of an album leaves it in
the library, so there are no bytes to free and nothing to exclude. The property it protected — a delete
path must not let the thing being deleted vote to keep its own bytes — moved to the library with the
purge, and `TestGlimtDeleteExcludesNoPhotograph` now asserts the complementary half (a glimt deletion
excludes no photograph). The exclusion argument itself is exercised by the library's delete path in task
379.

**"One shared purge helper" is now trivially satisfied** for the same reason: there is exactly one purge
path left, because the in-app album endpoints stopped purging. The two-doors hazard PRD 022 §8.9 raised
cannot arise between *those* two doors any more. It returns when task 379 adds the curator's photograph
deletion, and that task owns it.

## Acceptance Criteria

- [x] `RefsInUse` is answered by the `photo` projection, keyed on photo ids for the exclusion
- [x] `glimtdelete.go` and `albumremove.go` both compile against the new shape with no change in
      behaviour for glimt — asserted by the unchanged glimt-side tests
- [x] An error from any owner of the blob store aborts the purge and deletes nothing — tested, and
      verified by breaking it
- [x] A test proves a ref still referenced by a glimt is not purged when a photograph using the same
      bytes is deleted, and vice versa
- [x] ~~The exclusion regression test survives the reshaping~~ — superseded; see above
- [x] ~~The curator delete path and the in-app removal path call one shared purge helper~~ — only one
      purge path now exists; deferred to task 379 when the second returns
- [x] Every owner of the blob store is listed in the helper's doc comment, as it is today
