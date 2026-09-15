# 250 — `kort` + `kortsaet` projection

**Status:** doing
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15

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

- [ ] `go/nathejk/table/kort/` with `table.sql`, `kortsaet.sql`, `table.go`, `consumer.go`,
      `querier.go`.
- [ ] Patch semantics: an event carrying only `name` leaves `checkpointIds` and `extents`
      intact (test).
- [ ] Whole-record semantics: a `SetUpdated` without `teamType` clears it (test).
- [ ] `kort.sorted` / `kortsaet.sorted` set `sortOrder` by list position, leaving unnamed
      ids alone (test).
- [ ] A sheet arriving before its set is stored (test).
- [ ] Replaying the same events twice yields identical rows (test).
- [ ] Schema comments explain each hazard, not just the columns.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: follow the `checkpoint` package's shape exactly — `table.sql` +
  `kortsaet.sql` embedded, `table.go` with `New`, `consumer.go` folding events, `querier.go` for
  reads. Reuse its `quote`/`upsert`/`sortStrings` idioms rather than inventing new ones, since
  the patch-semantics hazard is identical.
