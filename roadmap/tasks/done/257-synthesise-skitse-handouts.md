# 257 — Synthesise `skitse` handouts from `handoutCheckgroupId`

**Status:** done
**Priority:** medium
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

PRD 016 §11.2. A `skitse` is a hand-drawn slip showing the next group of checkpoints. It
has **no QR code**, so it is never scanned, so no `qr.registered` event can ever name it —
which means it cannot appear in the `maphandout` projection at all. Yet it is physically
handed over at a post, and it does reveal checkpoints through its `handoutCheckgroupId`.
Its `checkpointIds` are its only trace in the system.

So: when the patrol has reached the checkgroup named by a sheet's `handoutCheckgroupId`,
that sheet is listed as a handout, synthesised rather than projected. This makes the handout
list say the same thing the reveal rule already acts on — a list that silently omits a sheet
the patrol is holding is worse than no list.

The synthesised entry has **no QR id** and **no "still held" state**: there is no binding to
lose. Its handout time is the patrol's scan at that checkgroup.

Applies to any sheet with a `handoutCheckgroupId`, not only skitser — a sheet handed out at
a post follows the same rule and would otherwise be missing too.

## Acceptance Criteria

- [x] A sheet with `handoutCheckgroupId` set is listed once the patrol has scanned at that
      checkgroup (test).
- [x] Not listed before that (test).
- [x] Synthesised entries carry no QR id and are not marked as "afleveret".
- [x] Handout time is the anchoring scan's time.
- [x] A sheet that is *both* QR-bound and has a handout checkgroup is not listed twice.
- [ ] Frontend renders a QR-less handout without an empty gap where the number goes.
      — deferred to task 268, which builds the drawer section. The BFF now marks such rows
      `Synthesised` so the client has something to branch on.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
- 2026-09-15 — Picked up. Plan: put the handout list beside the reveal rule in `internal/reveal`, since
  both answer "what has this patrol been given" from the same four inputs and the synthesis rule is the
  same predicate the reveal rule already evaluates. Keeping them apart would mean two implementations of
  one rule, which is how a list and a map start disagreeing.
- 2026-09-15 — `Rule.Handouts(year, patrolID)` written in `internal/reveal/handouts.go`. Two sources:
  recorded QR bindings, then synthesised post-handovers. Recorded ones are added **first** so a sheet that
  is both bound and keyed to a post keeps the real sticker number and timestamp rather than the inferred
  ones.
- 2026-09-15 — Decision: a synthesised handout's time is the **first** scan at the post, not the latest. A
  patrol re-scanning the same group is not handed a second copy, so the timestamp must not drift forward.
- 2026-09-15 — Decision: a synthesised handout reports `StillHeld: true` unconditionally. There is no
  binding, so there is nothing that could have been reassigned — and it is the truth: the patrol was handed
  a piece of paper and nobody took it back.
- 2026-09-15 — Decision: a sheet whose trigger names a **deleted** group synthesises nothing. Task 256
  makes such a sheet fall back to the QR rule, and a sheet under the QR rule with no binding was never
  handed over — inventing a handout would put a sheet in the patrol's list that nobody gave them.
- 2026-09-15 — Caught a case the task did not name: two handouts with an **unknown** sheet (`mapId == ""`)
  share the same empty id, so a naive dedupe would collapse them into one. They are two real handovers of
  two real pieces of paper. Unknown sheets are therefore never recorded as "listed", and there is a test.
- 2026-09-15 — Added `Synthesised` to the returned type so the client can lay a QR-less row out without an
  empty gap where the sticker number goes. That last acceptance criterion is frontend work and belongs to
  task 268; left unchecked here rather than quietly claimed.
- 2026-09-15 — Test note: `TestHandoutListAgreesWithTheRevealRule` asserts the list and the map together.
  They are computed from one predicate precisely so they cannot drift, and the drift a patrol would notice
  is the bad one — posts drawn on the map for a sheet the list says they were never handed.
- 2026-09-15 — ✅ Criteria complete apart from the frontend one. 31 tests in `internal/reveal`; build, vet,
  gofmt and the full suite clean.
- 2026-09-15 — Done. Moving to done/, with the frontend criterion carried into task 268.
