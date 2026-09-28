# 450 — photo.creditCrewId

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 025 §6 R1. The credit becomes either a crew reference or a typed string.

- `photo.creditPersonId VARCHAR(99) NOT NULL DEFAULT ""` beside the existing `credit`.
- A field on `photo.Updated` to match, with that event's pointer semantics.
- **Setting one clears the other**, in the handler *and* in the fold. A photograph with both set is a
  photograph with two answers about who took it, and the fold is the last place that can refuse one.

The id on the event log rather than the name is the entire point of PRD 025 (§8 D1): the log is append-only and
is never rewritten, so a name copied onto it could never be erased, while a reference can be deleted in one
place.

`isPersonShaped` currently rejects `creditpersonid` by name. It must be excepted **for the library scope only**
— the same treatment `filename` got in task 448 — so that a *public* response carrying the id still fails
(PRD 025 §6 R6). That exception belongs with task 451's guard work; this task only needs the column and the
event to exist.

## Acceptance Criteria

- [ ] Column, event field, fold
- [ ] Setting an id clears the typed credit, and vice versa — enforced in the fold as well as the handler
- [ ] Replaying a log written before this folds to ""
- [ ] The library read carries it

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R1.

## What changed

`photo.creditCrewId VARCHAR(99)`, `photo.Updated.CreditCrewID *string`, folded, and on the curator read.

**Named `creditCrewId`, not `creditPersonId`.** The narrower word is the true one — this only ever references a
crew member — and it keeps `person` meaning what it means everywhere else in the privacy guards. Note that the
guard caught it anyway, through the `credit` needle rather than `person`: task 393 added that needle precisely so
the credit family could not grow a field in silence, and it worked.

**One credit in force**, enforced in the fold as well as the handler: a typed name clears the reference and the
reference clears the typed name, each in its own branch. A row with both would be a photograph with two answers
about who took it, and the read would have to choose — a decision belonging to the curator, not to a COALESCE.
The fold does it too because the fold is what the projection actually believes; an event from an older publisher,
or a future one with a bug, must not be able to leave such a row behind.

Bounded at 99 runes to match the column, for the reason `credit` is: an over-long value is a MariaDB 1406 that
deadletters the message on **every** replay, which is the failure task 352 shipped and task 350 exists to prevent.

## The exception landed here rather than in task 451

Both privacy guards refused the column, as they should have:
`TestNoStructInTheLibraryOrTheAdminToolNamesAPerson` on the two struct fields, and
`TestTheLibraryTablesDeclareNoPersonShapedColumn` on the column. Task 451 owns the exception on paper, but a
column cannot be committed without it — the repo would be red — so `libraryPersonShapedExceptions` gained its
third entry here, with the full argument written where the exception lives: the erasure reasoning, the three
premises the maintainer corrected, and the bounds that still hold.

`TestTheCreditReferenceStaysOffThePublicSurface` is the new boundary test, modelled on the filename one: the word
is **still** person-shaped in general, so a public response carrying the id fails; and the exception is exactly
one name, so `creditPersonId`, `creditCrewName` and `creditCrewPhone` all still fail.

## Acceptance Criteria

- [x] Column, event field, fold
- [x] Setting an id clears the typed credit, and vice versa — enforced in the fold as well as the handler
      (the **handler** half is task 453's write path; the fold's half is here and guarded)
- [x] Replaying a log written before this folds to ""
- [x] The library read carries it

## Progress Log

- 2026-09-28 — Picked up alongside task 449.
- 2026-09-28 — Chose `creditCrewId` over `creditPersonId`; see above. The guard caught it either way, via the
  `credit` needle.
- 2026-09-28 — Both privacy guards refused it, so the exception had to land in the same commit. Written with the
  erasure argument and an explicit note that the option I argued for — storing the resolved name — is the one
  that cannot be undone.
- 2026-09-28 — ✅ `TestTheCreditFoldKeepsExactlyOneCreditInForce`, asserting each clear sits in the branch that
  sets the other field. Mutation-checked by deleting one clear.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean.
