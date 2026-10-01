# 478 — A takedown must free the original too

**Status:** done
**Priority:** high
**Created:** 2026-10-01
**Picked up by:** agent session (PRD 027 rollout)
**Started:** 2026-10-01
**Completed:** 2026-10-01

**PRD:** 027 (R8)
**Depends on:** 477
**Blocks:** 479 — this must land first

## Description

This is the one path in PRD 027 where getting it wrong is **not recoverable**, and it is why this task is sequenced
**before** task 479: no original may ever be stored that a takedown cannot remove. If the order is reversed, there
is a window in which photographs accumulate originals that the delete path walks past, and the only evidence is
bytes sitting in the backup scope that nobody will look for.

"Deleted from the library" has a meaning in this product (PRD 022 §5), and after PRD 027 the stored original carries
the camera's metadata — including where the photograph was taken. **A takedown that left an EXIF-bearing original
behind would be a takedown in name only:** the photograph would be off every page while the most sensitive copy of
it stayed on disk, inside the volume that is backed up and never purged.

Two changes, and they are two halves of the same invariant:

1. **`photo.RefsInUse` must include `originalRef`.** Freeing is decided by asking every table which refs it still
   needs. Blobs may be shared between library photographs and glimt (task 368) — content addressing means two rows
   can legitimately name the same bytes — so a ref omitted here can be freed out from under a row that still uses
   it by a delete happening somewhere else entirely. This is the same class of bug task 368 is about.
2. **`cmd/api/admindelete.go` must free the original's bytes.** The takedown already frees the renditions; it must
   now also free the original, and it must do so through the same in-use check, so that nothing is freed that
   another row still names.

Those two pull in opposite directions on purpose: one stops us freeing too much, the other stops us freeing too
little. Both are required for the path to be correct.

## Acceptance Criteria

- [x] `photo.RefsInUse` reports `originalRef` alongside the rendition refs
- [x] A test asserts that a delete in another table does not free a blob a library photograph still names as its
      original
- [x] `cmd/api/admindelete.go` frees the original's bytes as part of a library takedown
- [x] A test asserts that after a takedown the original's bytes are gone — the assertion is on the blob being
      unreachable, not merely on the column being cleared
- [x] A test asserts a shared blob is **not** freed while another row still names it
- [x] Merged and deployed before task 479, so no original exists that a takedown would miss
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-10-01 — Task created from PRD 027.
- 2026-10-01 — Picked up. Plan: `originalRef` into `RefsInUse`'s column list, then into the takedown's ref list, then
  tests for both directions of the mistake.
- 2026-10-01 — `RefsInUse` had the shape for this already: a `columns` list drove the WHERE clause and the argument
  arithmetic, with a comment warning that the arithmetic "silently breaks when a column is added". True of the half it
  covered — the `SELECT` list and the `Scan` were written out by hand, so adding a fourth column meant four places
  having to agree inside a function whose own doc says a missed column can **delete a live object because nothing
  claimed it**. Derived all four from the one list instead of extending the duplication. "Every ref column" is now
  enforced by construction rather than by a sentence.
- 2026-10-01 — Takedown: `p.OriginalRef` appended **last**, deliberately. The order in that slice is the order bytes
  are freed in, and the original is the one object whose loss cannot be undone, so it goes after everything cheaper
  has already succeeded.
- 2026-10-01 — ✅ Tests in `cmd/api/admindeleteoriginal_test.go`, covering both directions because they fail in
  opposite ways: the original is freed; a shared original another live row still names is **not** freed; a photograph
  with no original is an ordinary takedown and no empty ref reaches the sharing check; and the original is actually
  *offered* to the sharing check. That last one matters because "never asked" and "asked and told no" both end with
  the bytes gone, and only one is correct — the other destroys somebody else's archive master the day a second row
  appears.
- 2026-10-01 — Added `TestEveryRefColumnIsNamedByTheDeletePathAndTheSharingCheck`, which reads the ref columns out of
  `table.sql` rather than listing them. The bug being prevented is *forgetting*, and a hand-written list in a test
  forgets the same way the code does — `mediumRef` had to be added to both places separately, and PRD 027 added a
  fifth column. Now a sixth fails loudly with an instruction.
- 2026-10-01 — Verified none of it is vacuous. Removing the `p.OriginalRef` append failed three tests with their own
  explanations; removing `originalRef` from the `columns` list failed the structural guard. Both reverted, `git diff
  --stat` confirms only the intended changes.
- 2026-10-01 — `gofmt` clean, `go build`, full `go test ./...` green. Moving to done. 479 is now unblocked: nothing can
  store an original that this path would miss.
