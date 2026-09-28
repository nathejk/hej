# 452 — The public and admin reads render the resolved credit

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 025 §6 R5/R6. Needs task 451.

The public album page and the curator's reads show the credit whether it came from the picker or from typed
text — they are the same credit line by two routes, and the page must not be able to tell.

- **The public response carries the name, never the id** (R6). The id is a handle to a person record and has no
  business on the open web, even though it names nobody by itself. `isPersonShaped` still flags it, so a public
  type that grows one fails — see task 451.
- **An unresolvable id renders nothing** (R5), which is what a photograph with no credit already does.
- The album page is cached for 60 seconds, so a deleted crew member stops being named within that window. That
  is the erasure path working, and it is worth a line in the page's own comments.

## Acceptance Criteria

- [ ] The public album page credits a picked photographer identically to a typed one
- [ ] No public response carries `creditPersonId`
- [ ] A photograph whose credit id resolves to nothing renders no credit line
- [ ] The curator's reads show the resolved name, so the tool shows what the public will see

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R5/R6.
