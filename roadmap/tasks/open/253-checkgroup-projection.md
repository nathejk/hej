# 253 — `checkgroup` projection

**Status:** open
**Priority:** high
**Created:** 2026-09-15

## Description

PRD 016 phase 1. New projection from `NathejkCheckgroupUpdated` / `NathejkCheckgroupDeleted`
and `NathejkCheckgroupsSorted`. Columns:

- `name`
- `sortOrder` — the outer half of route order (`checkgroup.sortOrder`,
  `checkpoint.sortOrder`) used by the next-checkpoint arrows (PRD 016 §11.9)
- `scheme` — `fixed` | `relative` | `none`
- `relativeCheckgroupId` — with `scheme: relative`, the checkgroup whose scan anchors the
  window

`scheme` + `relativeCheckgroupId` are what make a relative window resolvable at all
(PRD 016 §11.5): `relative` means "open for `openDuration` minutes from this patrol's own
scan at the named checkgroup", and we hold that scan. Without these two columns every
relative checkpoint would silently lose its verdict.

**`showOnMap` is deliberately NOT consumed.** It is written by hq's `PostlinjeModal`
toggle and read by nothing in hq — its intent is unverified, and both ways of guessing are
bad. Our reveal rule is grounded in physical possession instead, which cannot over-reveal
whatever the flag means (PRD 016 §11.3). Leave a comment saying so, so the next person
does not "fix" the omission.

Note `NathejkCheckgroupUpdated` is a **patch** (pointer fields) like the checkpoint one.

## Acceptance Criteria

- [ ] `go/nathejk/table/checkgroup/` with `table.sql`, `table.go`, `consumer.go`,
      `querier.go`.
- [ ] Patch semantics honoured (a rename does not clear `scheme`).
- [ ] `NathejkCheckgroupsSorted` sets `sortOrder` by list position.
- [ ] Delete is a soft delete, and a later update restores (matching `checkpoint` and
      `person`).
- [ ] A comment records why `showOnMap` is not consumed, referencing PRD 016 §11.3.
- [ ] Idempotent under replay (test).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 1.
