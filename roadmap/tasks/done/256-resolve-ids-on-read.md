# 256 — Resolve `checkpointIds` and dangling `handoutCheckgroupId` on read

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

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

- [x] Unresolvable `checkpointIds` are dropped silently on read (test).
- [x] A checkgroup deletion removes its checkpoints from every sheet's effective list
      without any per-checkpoint event (test — this is the case that motivated the fix).
- [x] A dangling `handoutCheckgroupId` falls back to the QR rule (test).
- [x] Comments state that neither fix travels over the stream, so the next consumer knows.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
- 2026-09-15 — Picked up. Half of this is already done by construction (task 255's log): `ByIDs` cannot
  return a checkpoint that no longer exists, so unresolvable ids are dropped by the query itself. The
  remaining work is the **checkgroup** half — a dangling `handoutCheckgroupId` must read as `""`, the QR
  rule — plus tests pinning both, including the checkgroup-deletion case that motivated the whole fix.
- 2026-09-15 — Added `Checkgroups` as a fifth input to the rule, purely to know which groups exist. Nothing
  else needs it: the reveal rule already gets reached groups from the scans.
- 2026-09-15 — Implemented the fallback in `revealsFor`: a trigger naming an unknown group is treated as
  `""`, so the sheet reverts to the QR rule.

  Worth recording *why* that direction, because the asymmetry is not obvious. Keyed to a deleted post, a
  sheet's checkpoints would never appear **at all** — a sheet in the patrol's hand whose posts the app
  refuses to draw, forever, with nothing in any log to explain it. Falling back can at worst reveal the
  sheet to a patrol that was handed that sheet, which is the QR rule working as intended. One failure is
  silent and permanent; the other is bounded by possession. HQ's own read path makes the same choice.
- 2026-09-15 — Both fixes are commented as **not travelling over the stream**, since any other consumer of
  the kort events has to implement them independently — the contract says so and it is easy to miss.
- 2026-09-15 — Test note: `TestDanglingHandoutCheckgroupFallsBackToTheQRRule` has both halves, because the
  fallback must be the QR rule and *not* an unconditional reveal. A test that only checked the handed-out
  case would pass against an implementation that revealed the sheet to everybody.
- 2026-09-15 — ✅ All criteria complete. 18 tests in `internal/reveal`; build, vet, gofmt and the full
  suite clean.
- 2026-09-15 — Done. Moving to done/.
