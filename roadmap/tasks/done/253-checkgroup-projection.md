# 253 — `checkgroup` projection

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

PRD 016 phase 1. New projection from `NathejkCheckgroupUpdated` / `NathejkCheckgroupDeleted`
and `NathejkCheckgroupsSorted`. Columns:

- `name`
- `sortOrder` — the outer half of route order (`checkgroup.sortOrder`,
  `checkpoint.sortOrder`) used by the next-checkpoint arrows (PRD 016 §11.9)
- `scheme` — `fixed` | `relative` | `none`
- `relativeCheckgroupId` — with `scheme: relative`, the checkgroup whose scan anchors the
  window

`scheme` + `relativeCheckgroupId` are what make a relative window resolvable at all
(PRD 016 §11.5): `relative` means "open for `openDuration` minutes from this patrol's own
scan at the named checkgroup", and we hold that scan. Without these two columns every
relative checkpoint would silently lose its verdict.

**`showOnMap` is deliberately NOT consumed.** It is written by hq's `PostlinjeModal`
toggle and read by nothing in hq — its intent is unverified, and both ways of guessing are
bad. Our reveal rule is grounded in physical possession instead, which cannot over-reveal
whatever the flag means (PRD 016 §11.3). Leave a comment saying so, so the next person
does not "fix" the omission.

Note `NathejkCheckgroupUpdated` is a **patch** (pointer fields) like the checkpoint one.

## Acceptance Criteria

- [x] `go/nathejk/table/checkgroup/` with `table.sql`, `table.go`, `consumer.go`,
      `querier.go`.
- [x] Patch semantics honoured (a rename does not clear `scheme`).
- [x] `NathejkCheckgroupsSorted` sets `sortOrder` by list position.
- [x] Delete is a soft delete, and a later update restores (matching `checkpoint` and
      `person`).
- [x] A comment records why `showOnMap` is not consumed, referencing PRD 016 §11.3.
- [x] Idempotent under replay (test).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: new `go/nathejk/table/checkgroup/` following the `checkpoint`
  package's shape. Subjects `checkgroup.*.updated`, `checkgroup.*.deleted`, `checkgroups.sorted`
  (note the plural on the sorted subject — confirmed against hq rather than guessed).
- 2026-09-15 — The subject spellings are worth recording, because they are not one convention: the
  group reorder is `checkgroups.sorted` (plural, collection-level) while the checkpoint reorder is
  `checkgroup.{id}.checkpoints_sorted` (singular, per group). Neither is derivable from the other, so
  `TestSortedSubjectIsPlural` pins both.
- 2026-09-15 — Projected `scheme` and `relativeCheckgroupId` as well as name and sortOrder. These two
  are what make a relative window resolvable at all — without them every relative checkpoint would
  silently lose its verdict, which was open question §11.5 before the code answered it.
- 2026-09-15 — Decision: an **unrecognised `scheme` is stored as sent** and yields no verdict, the
  same outcome as `none`. Rejecting or defaulting it would mean inventing a verdict from a scheme we
  do not understand, and a wrong "for sent" is worse than a missing one.
- 2026-09-15 — Decision: `relativeCheckgroupId` **is** written when empty, unlike most fields.
  Clearing the anchor is a real edit; a stale anchor would measure the window from a group the
  organizer no longer intends, which is a wrong verdict rather than a missing one.
- 2026-09-15 — Decision: a checkgroup delete does **not** cascade to its checkpoints, matching what
  the stream does (no per-checkpoint event) and leaving the dangling reference to task 256's read-time
  resolution. Cascading would couple two independent projections' replay order, and getting that wrong
  deletes checkpoints the race area is derived from. Test asserts the delete never touches the
  checkpoint table.
- 2026-09-15 — `showOnMap` and `mandatory` are on the event and deliberately unprojected, with the
  reasoning at the foot of `table.sql` and a test (`TestShowOnMapIsNotStored`) that fails if either
  reaches a statement. The point is that a future reader finds a decision rather than an oversight.
- 2026-09-15 — ✅ All criteria complete. 13 tests; build, vet, gofmt and the full suite clean.
- 2026-09-15 — Done. Moving to done/. Phase 1 has one task left: 254 (`scan` + `checkpersonnel`),
  which is the one that retires the seeded scans mock.
