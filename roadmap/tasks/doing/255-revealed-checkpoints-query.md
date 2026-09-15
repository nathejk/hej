# 255 — `RevealedCheckpoints(year, patrolID)` with a publishable-only type

**Status:** doing
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15

## Description

PRD 016 phase 2 — the security boundary of the whole feature.

Implement the reveal rule: the union of

1. the `checkpointIds` of every sheet **ever handed to the patrol** whose reveal rule is
   the QR rule (`handoutCheckgroupId == ""`);
2. the `checkpointIds` of every sheet whose `handoutCheckgroupId` names a checkgroup the
   patrol has **already reached**;
3. every checkpoint in a checkgroup the patrol has **already scanned**.

The three do not nest and none can be derived from another (vendored contract §1): a
skitse shows a subset of one checkgroup, a double-sided A3 spans two.

**Revealing is monotonic** (PRD 016 §11.4): nothing already revealed is withdrawn, even
when a sheet is reassigned to another team. The knowledge left with the scout, not the
sheet, so un-revealing achieves no secrecy and makes the map lie about ground already
walked.

**Shape matters more than the query.** Follow the precedent set by `checkpoint.Queries`,
which exposes only the race-area hull *precisely so that no call site can leak a position*:

- the method takes the patrol id and returns only that patrol's revealed checkpoints;
- the return type contains only publishable fields;
- there must be **no way** for a handler to ask for all checkpoints.

Only positioned, non-deleted checkpoints are returned, each with id, name, checkgroup,
sort order and window. Scoped to sheets in the `patrulje` map set(s) (task 267's filter,
shared here).

## Acceptance Criteria

- [ ] `RevealedCheckpoints(ctx, year, patrolID)` added to `checkpoint.Queries`.
- [ ] Return type carries no field that must not reach a participant.
- [ ] Each of the three rules has its own test, plus one where they overlap.
- [ ] Monotonicity test: a reassigned sheet's checkpoints stay revealed.
- [ ] A patrol with no handouts and no scans gets an empty list, not an error.
- [ ] `RaceArea` untouched.
- [ ] Doc comment states why the interface, not the handler, is the boundary.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
- 2026-09-15 — Picked up. Reading the four projections' read APIs first, to decide where the rule lives:
  it needs sheets (kort), handouts (maphandout), reached checkgroups (scan) and the checkpoints
  themselves, and the `nathejk/table/*` packages are bound for shared-go and must not know about each
  other.
