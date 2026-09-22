# 368 — The shared-blob check asks the library, and still refuses to guess

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

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

## Acceptance Criteria

- [ ] `RefsInUse` is answered by the `photo` projection, keyed on photo ids for the exclusion
- [ ] `glimtdelete.go` and `albumremove.go` both compile against the new shape with no change in
      behaviour for glimt
- [ ] An error from any owner of the blob store aborts the purge and deletes nothing — tested
- [ ] A test proves a ref still referenced by a glimt is not purged when a photograph using the same
      bytes is deleted, and vice versa
- [ ] The exclusion regression test survives the reshaping: removing a row does not see itself
- [ ] The curator delete path and the in-app removal path call one shared purge helper
- [ ] Every owner of the blob store is listed in the helper's doc comment, as it is today
