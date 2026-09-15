# 255 — `RevealedCheckpoints(year, patrolID)` with a publishable-only type

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

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

- [x] The reveal rule implemented, in `go/internal/reveal` rather than on `checkpoint.Queries`
      (see the log; PRD 016 §8 amended to match).
- [x] Return type carries no field that must not reach a participant.
- [x] Each of the three rules has its own test, plus one where they overlap.
- [x] Monotonicity test: a reassigned sheet's checkpoints stay revealed.
- [x] A patrol with no handouts and no scans gets an empty list, not an error.
- [x] `RaceArea` untouched.
- [x] Doc comment states why the interface, not the handler, is the boundary.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
- 2026-09-15 — Picked up. Reading the four projections' read APIs first, to decide where the rule lives:
  it needs sheets (kort), handouts (maphandout), reached checkgroups (scan) and the checkpoints
  themselves, and the `nathejk/table/*` packages are bound for shared-go and must not know about each
  other.
- 2026-09-15 — **Deviation from PRD 016 §8, with the PRD amended to match.** The PRD sketched this as
  `checkpoint.Queries.RevealedCheckpoints`. That cannot work: the rule needs four projections at once, and
  whichever one hosted it would have to read another's tables — breaking the shared-go-bound isolation
  those packages keep. The rule now lives in `go/internal/reveal`.
- 2026-09-15 — The security property is kept, one layer out, and this is the design decision of the task.
  `checkpoint.Queries` gained **two bounded reads** instead of one broad one: `ByIDs(year, ids)` and
  `ByCheckgroups(year, groups)`. Neither can return anything the caller did not already name, so there is
  still no way to ask the projection for all checkpoints. `internal/reveal` decides what a patrol has
  earned; the projection refuses every broader question. An empty id list returns nothing and runs no
  query — treating "named nothing" as "everything" is exactly how a bounded read silently becomes
  unbounded, so it is refused explicitly rather than left to SQL's `IN ()`.
- 2026-09-15 — Rules 1 and 2 are kept apart from rule 3 all the way to the end, rather than collapsed
  into one id set early. The contract is explicit that the reveal units "do not nest and none can be
  derived from another", and collapsing them is how that property gets quietly lost — a skitse shows a
  subset of one checkgroup, a double-sided A3 spans two.
- 2026-09-15 — Decision: rule 2 requires **no handout record**, only that the patrol reached the sheet's
  handout checkgroup. It cannot require one: a skitse has no QR code, so no binding event can ever name
  it, and demanding a record would make every skitse permanently invisible. That is the mistake the
  contract's two-rules warning exists to prevent, and it now has a test.
- 2026-09-15 — Decision: `Handout.Current` is **ignored** by the rule. It says whether the patrol still
  holds the code, which matters for how the handout list is *labelled* and not at all for what has been
  revealed — revealing is monotonic (§11.4).
- 2026-09-15 — Test design worth noting: the checkpoint fake records **which ids and groups it was asked
  for**, and the fixture world contains a `cp-secret` that no rule reveals. So
  `TestOnlyRevealedIdsAreEverAskedFor` fails if the rule asks a broader question than it should — a fake
  that returned everything regardless would let the result assertions pass while the real query leaked.
- 2026-09-15 — Decision: an input failure is **returned**, not treated as "nothing revealed". An empty map
  is a legitimate state, so a swallowed error would be indistinguishable from a patrol that has scanned
  nothing — and the client would cache that emptiness offline.
- 2026-09-15 — ✅ All criteria complete. 15 tests in `internal/reveal`, all running without a database
  (the rule is the interesting part, so it is testable in isolation). Build, vet, gofmt and the full suite
  clean.
- 2026-09-15 — Done. Moving to done/. Note for task 256: unresolvable checkpoint ids are already dropped
  by `ByIDs` (a row that does not exist cannot be returned), so that task's remaining work is the
  *checkgroup* half — a dangling `handoutCheckgroupId` reading as the QR rule — plus the tests that pin
  both.
- 2026-09-15 — **Follow-up: I committed before running the full suite, and it was broken.** Widening
  `checkpoint.Queries` made an existing test double in `cmd/api/racearea_test.go` stop satisfying it. My
  mistake in process — the package tests passed and I took that for the suite.

  Fixed by improving the design rather than patching the fake: `checkpoint.AreaQueries` (RaceArea only) is
  now split out, `Queries` embeds it, and `data.Models.RaceAreas` is typed as the narrow one. The
  race-area handler needed nothing else, and a handler that depends on the whole projection is a handler
  that *could* read checkpoint positions — so narrowing the dependency is the same discipline as bounding
  the reads. It also removes the pressure on every existing double to implement reads it has no opinion
  about, which is how fakes drift into fiction. Suite, vet and gofmt clean.
