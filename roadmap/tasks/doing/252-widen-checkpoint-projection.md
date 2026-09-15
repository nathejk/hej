# 252 — Widen the `checkpoint` projection: checkgroup, sort order, window

**Status:** doing
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15

## Description

PRD 016 phase 1. The `checkpoint` projection currently stores name and position only, and
discards three things that are already on the stream and that PRD 016 needs:

- **`checkgroupId`** — from `NathejkCheckpointCreated`, a subject this repo does **not
  consume at all** today (the consumer's comment says `.created` "carries nothing this
  projection wants"; that is no longer true, and the comment must be updated rather than
  left to mislead). Needed for reveal rule 3: scanning a checkpoint reveals its whole
  checkgroup.
- **`sortOrder`** — from `NathejkCheckpointsSorted`. Position in the list is the order.
  Half of the route order for the next-checkpoint arrows (the other half is the
  checkgroup's, task 253).
- **The open window** — `FixedTimeRange` → `openFromUts` / `openUntilUts`;
  `RelativeTimeDuration` → `openDuration` **in minutes**. Drives the on-time verdict.

Keep every existing property of the projection:

- `handleUpdated` writes **only the columns the event carries** — every field on
  `NathejkCheckpointUpdated` is a pointer and nil means unchanged. A rename must not erase
  a position, and now must not erase a window either.
- `0,0` stays "no position" (it is the Atlantic off Ghana, and it is what an unset
  coordinate serialises to).
- Idempotent upserts; replay-safe.
- `RaceArea` must keep working exactly as before — it is the only thing that may expose
  anything derived from *unrevealed* checkpoints, and only as a hull.

## Acceptance Criteria

- [ ] Columns added: `checkgroupId`, `sortOrder`, `openFromUts`, `openUntilUts`,
      `openDuration`.
- [ ] `NathejkCheckpointCreated` consumed for `checkgroupId`; the stale comment about
      `.created` carrying nothing is corrected.
- [ ] `NathejkCheckpointsSorted` consumed; unnamed ids keep their order.
- [ ] A partial `updated` still leaves untouched columns alone (test extended to cover the
      new ones).
- [ ] `RaceArea` behaviour unchanged (existing tests pass untouched).
- [ ] Schema comments explain why each new column exists and what reads it.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: add the five columns to `table.sql`, subscribe to
  `checkpoint.*.created` for `checkgroupId` and `checkpoint.sorted` for `sortOrder`, extend
  `handleUpdated` for the window — keeping its "only the columns the event carries" property intact,
  since that is what stops a rename erasing a position.
