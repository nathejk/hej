# 256 — Resolve `checkpointIds` and dangling `handoutCheckgroupId` on read

**Status:** open
**Priority:** high
**Created:** 2026-09-15

## Description

PRD 016 phase 2. Two referential-integrity fixes that **do not travel over the stream** and
that we must therefore implement ourselves (vendored contract §4 and §1.1).

1. **Resolve `checkpointIds` against our own checkpoint projection and drop ids that do not
   resolve.** A `kort` event carries the ids that were saved, and nothing re-publishes them
   when a checkpoint later disappears. In particular **deleting a checkgroup emits no
   per-checkpoint event**, so ids inside the JSON array cannot be cascaded out. hq's own
   read path filters on read; the fix is not on the stream, so ours must do the same.

   Filtering on read (rather than trying to prune the array on write) also self-heals every
   other cause of a stale id — a checkpoint deleted while we were down, a half-finished
   replay — without depending on the order two independent projections happen to run in.

2. **A `handoutCheckgroupId` naming a checkgroup that no longer exists reads as `""`, the
   QR rule.** That is the safe direction: the alternative is a reveal keyed to a post that
   will never be reached, so those checkpoints would never appear at all. hq does exactly
   this.

Both are cheap (tens of rows) and belong next to the reveal rule (task 255).

## Acceptance Criteria

- [ ] Unresolvable `checkpointIds` are dropped silently on read (test).
- [ ] A checkgroup deletion removes its checkpoints from every sheet's effective list
      without any per-checkpoint event (test — this is the case that motivated the fix).
- [ ] A dangling `handoutCheckgroupId` falls back to the QR rule (test).
- [ ] Comments state that neither fix travels over the stream, so the next consumer knows.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
