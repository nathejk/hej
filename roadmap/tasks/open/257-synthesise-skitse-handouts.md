# 257 — Synthesise `skitse` handouts from `handoutCheckgroupId`

**Status:** open
**Priority:** medium
**Created:** 2026-09-15

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

- [ ] A sheet with `handoutCheckgroupId` set is listed once the patrol has scanned at that
      checkgroup (test).
- [ ] Not listed before that (test).
- [ ] Synthesised entries carry no QR id and are not marked as "afleveret".
- [ ] Handout time is the anchoring scan's time.
- [ ] A sheet that is *both* QR-bound and has a handout checkgroup is not listed twice.
- [ ] Frontend renders a QR-less handout without an empty gap where the number goes.

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 2.
