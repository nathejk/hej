# 252 — Widen the `checkpoint` projection: checkgroup, sort order, window

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

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

- [x] Columns added: `checkgroupId`, `sortOrder`, `openFromUts`, `openUntilUts`,
      `openDuration`.
- [x] `NathejkCheckpointCreated` consumed for `checkgroupId`; the stale comment about
      `.created` carrying nothing is corrected.
- [x] `NathejkCheckpointsSorted` consumed; unnamed ids keep their order.
- [x] A partial `updated` still leaves untouched columns alone (test extended to cover the
      new ones).
- [x] `RaceArea` behaviour unchanged (existing tests pass untouched).
- [x] Schema comments explain why each new column exists and what reads it.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
- 2026-09-15 — Picked up. Plan: add the five columns to `table.sql`, subscribe to
  `checkpoint.*.created` for `checkgroupId` and `checkpoint.sorted` for `sortOrder`, extend
  `handleUpdated` for the window — keeping its "only the columns the event carries" property intact,
  since that is what stops a rename erasing a position.
- 2026-09-15 — Correction to the plan: the checkpoints-reorder subject is **not** under
  `checkpoint`. It is `NATHEJK.*.checkgroup.{checkgroupId}.checkpoints_sorted` — published against
  the group whose checkpoints moved. Found by reading hq's consumer rather than guessing.
- 2026-09-15 — **Upstream bug found, and deliberately not copied.** hq's handler for
  `checkpoints_sorted` does `DELETE FROM checkpoint WHERE checkgroupId=<Parts()[1]>` and applies no
  order at all. `Parts()[1]` is the **year**, not the checkgroup id, so the delete matches nothing
  and the handler is in practice a no-op. Two separate defects. Copying either would be worse here
  than upstream: this table's positions are what the offline map's race area is derived from, so a
  reorder that deleted a checkgroup's checkpoints would silently shrink the cached region — exactly
  the failure `handleUpdated`'s partial-write logic exists to prevent. Implemented what the event
  *means* instead (position = order), with a test asserting a reorder never emits a DELETE. Worth
  reporting to HQ.
- 2026-09-15 — An existing test, `TestCreatedIsNotConsumed`, failed — correctly. It encoded the
  earlier decision that `.created` "carries nothing this projection wants", which was true while the
  only reader was the race area. What changed is the reader, not the event. Inverted it into
  `TestCreatedIsConsumedForItsCheckgroup` rather than deleting it, so the reversal is recorded and
  the subscription is now pinned in the other direction — drop it and every reveal-by-checkgroup
  silently stops working.
- 2026-09-15 — Decision: fixed and relative windows are stored **separately**, not merged. A fixed
  window is a pair of instants; a relative one is a duration whose anchor is the patrol's own scan at
  another checkgroup, which this projection cannot see. Merging would mean inventing an anchor or
  losing the distinction the verdict logic needs.
- 2026-09-15 — **Caught a real deployment bug in my own work.** `CREATE TABLE IF NOT EXISTS` is a
  no-op against a database that has already booted, so the five new columns would never have existed
  on the live deployment and the first read would have failed at the query. The repo already has the
  answer — `cqrs.EnsureColumn` / `EnsureIndex`, used by the person package with an "additive drift
  only" comment — so `New` now brings an existing table forward. The new `kort`/`kortsaet`/
  `maphandout` tables need no equivalent: no deployment has them yet.
- 2026-09-15 — ✅ All criteria complete. `RaceArea`'s own tests pass untouched, which was the point
  of widening rather than rewriting. Full suite, `go vet ./...` and `gofmt` clean.
- 2026-09-15 — Done. Moving to done/.
