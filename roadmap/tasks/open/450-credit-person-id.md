# 450 — photo.creditPersonId

**Status:** open
**Priority:** high
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 025 §6 R1. The credit becomes either a crew reference or a typed string.

- `photo.creditPersonId VARCHAR(99) NOT NULL DEFAULT ""` beside the existing `credit`.
- A field on `photo.Updated` to match, with that event's pointer semantics.
- **Setting one clears the other**, in the handler *and* in the fold. A photograph with both set is a
  photograph with two answers about who took it, and the fold is the last place that can refuse one.

The id on the event log rather than the name is the entire point of PRD 025 (§8 D1): the log is append-only and
is never rewritten, so a name copied onto it could never be erased, while a reference can be deleted in one
place.

`isPersonShaped` currently rejects `creditpersonid` by name. It must be excepted **for the library scope only**
— the same treatment `filename` got in task 448 — so that a *public* response carrying the id still fails
(PRD 025 §6 R6). That exception belongs with task 451's guard work; this task only needs the column and the
event to exist.

## Acceptance Criteria

- [ ] Column, event field, fold
- [ ] Setting an id clears the typed credit, and vice versa — enforced in the fold as well as the handler
- [ ] Replaying a log written before this folds to ""
- [ ] The library read carries it

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R1.
