# 072 — Project spejder (and patrulje team names)

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §8. Consume `NATHEJK.*.spejder.*.updated` / `.deleted` and the patrulje
subjects (`signedup`, `updated`, `started`) into the person projection.

Spejder are the only population with a **guardian phone** (`PhoneParent`), which
PRD 005's confirmation step depends on — the model must make "not applicable"
distinguishable from "missing".

Consume events directly rather than reading shared-go's `spejder` projection: the
survey found `spejder.GetByID` is a stub returning `nil, nil` and `GetAll` joins
`spejderstatus`, which shared-go does not own.

## Acceptance Criteria

- [ ] `Consumes()` lists the spejder + patrulje subjects
- [ ] `HandleMessage` is idempotent (replayed on every boot)
- [ ] Guardian phone stored, and "not applicable" distinguishable from "missing"
- [ ] Team id and name populated from the patrulje events
- [ ] Member status carried (PRD 005's skip rule reads it)
- [ ] Tests using `cqrstest` fakes, no database required

## Progress Log

- 2026-08-25 — Task created from PRD 006.
