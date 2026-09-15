# 250 — `kort` + `kortsaet` projection

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

PRD 016 phase 1. Build the read model for map sheets and map sets from the stream, using
the mirror types from task 249. Subjects:

```
NATHEJK.*.kort.*.{created,updated,deleted}      NATHEJK.*.kort.sorted
NATHEJK.*.kortsaet.*.{created,updated,deleted}  NATHEJK.*.kortsaet.sorted
```

Copied from hq's schema rather than shared with it (PRD 016 §8) — including its schema
comments, which record hazards learned the hard way. Copy the *reasoning*, not just the
SQL.

Hazards that must be handled, all documented in the vendored contract:

- **A sheet's `updated` is a patch.** Writing all columns unconditionally lets a rename
  erase a checkpoint list. This repo's `checkpoint` consumer already solves exactly this
  — follow it (only write the columns the event carries).
- **A set's `created`/`updated` is a whole record.** An absent `teamType` must write NULL,
  not leave the old value — otherwise un-marking a set is inexpressible.
- **`…sorted` names only the ids being placed**; unnamed ids keep their order.
- **A sheet may materialise before its set** (stream order), so an unknown `kortsaetId` is
  tolerated, never dropped.
- **`checkpointIds` is a JSON array** in a TEXT column, read in one direction only
  (given a sheet, which checkpoints).
- Statements must be idempotent upserts: projections replay from sequence zero on boot.

## Acceptance Criteria

- [x] `go/nathejk/table/kort/` with `table.sql`, `kortsaet.sql`, `table.go`, `consumer.go`,
      `querier.go`.
- [x] Patch semantics: an event carrying only `name` leaves `checkpointIds` and `extents`
      intact (test).
- [x] Whole-record semantics: a `SetUpdated` without `teamType` clears it (test).
- [x] `kort.sorted` / `kortsaet.sorted` set `sortOrder` by list position, leaving unnamed
      ids alone (test).
- [x] A sheet arriving before its set is stored (test).
- [x] Replaying the same events twice yields identical rows (test).
- [x] Schema comments explain each hazard, not just the columns.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: follow the `checkpoint` package's shape exactly — `table.sql` +
  `kortsaet.sql` embedded, `table.go` with `New`, `consumer.go` folding events, `querier.go` for
  reads. Reuse its `quote`/`upsert`/`sortStrings` idioms rather than inventing new ones, since
  the patch-semantics hazard is identical.
- 2026-09-15 — Schemas written. Both carry the upstream reasoning rather than just the columns:
  why `checkpointIds` is a JSON array and not a join table, why `handoutCheckgroupId` defaults to
  `""`, why `teamType` is NULL-able and not unique.
- 2026-09-15 — Generalised `upsert` to take a table name and key columns, since two tables now
  share it. Otherwise the idiom is `checkpoint`'s unchanged.
- 2026-09-15 — Decision: **dispatch tests `.sorted` before `{id}.{verb}`**. `kort.sorted` carries
  no entity id, and if a matcher read "sorted" as an id, a reorder would be folded as an update to
  a phantom sheet named "sorted" — creating a junk row *and* losing the reorder. Ordering the
  switch makes the dispatch independent of how the matcher treats it, and there is a test
  (`TestSortedIsNotMistakenForAnEntityEvent`) pinning it.
- 2026-09-15 — Decision: an undecodable body is **reported and errored**, not silently skipped —
  via a `ReportUnknownBody` option, following `checkpoint.ReportPositionless`'s shape. These types
  are mirrored from another repo's unstable shapes, so a body we cannot read is the only signal the
  mirror has drifted (PRD 016 §11.11). Unknown *fields* still pass silently, which is the opposite
  case and deliberate.
- 2026-09-15 — Added a `ReportCounts` option for task 260 to wire: sets, patrol sets, sheets,
  patrol sheets. "12 sheets, 0 in a patrulje set" is this feature's characteristic disaster and is
  only visible in aggregate.
- 2026-09-15 — Deletes are soft, and a later create/update restores (`deleted: 0` written on both)
  — matching `checkpoint` and `person`, where the last event wins.
- 2026-09-15 — Cleanup: removed two things I had written and did not need — a `var _ = fmt.Sprintf`
  keeping an unused import alive, and a doc reference to a `Kortsaet` type that belongs to a later
  task. Both would have read as deliberate to the next person.
- 2026-09-15 — ✅ All criteria complete. 21 tests in the package; `go build ./...`, `go vet ./...`,
  `gofmt -l` and the full `go test ./...` suite all clean.
- 2026-09-15 — Note for task 267: `querier.PatrolSheets` filters on `s.teamType = 'patrulje'`
  joined across **all** matching sets, never on the set's name. Its tests (a renamed set changing
  nothing; two patrol sets both matching) are that task's acceptance criteria, so they are written
  there against the endpoint rather than duplicated here.
- 2026-09-15 — Done. Moving to done/; tasks 251–254 (the remaining phase-1 projections) are
  unblocked and independent of each other.
