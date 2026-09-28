# 454 — A credit filter in the library

**Status:** open
**Priority:** low
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 025 §6 R7. **Lower priority than the rest of PRD 025, and the reason is worth recording:** its original
justification was repairing misspelled credits already in the 2026 data, and the maintainer confirmed there are
none — the feature has not been used yet.

It stays on the board because "which photographs are credited to X" is a question that will be asked — to
re-credit a batch, or to answer a photographer's own request about their work — and today the library cannot
answer it. The filter composes with the others, like `album`, `location`, `verdict` and `tagged`.

## Acceptance Criteria

- [ ] A credit filter on the library read, composing with the existing ones
- [ ] Reachable from the contact sheet's filter row
- [ ] OpenAPI updated
- [ ] Works for both a picked credit and a typed one

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R7, at low priority: nothing to repair yet.
