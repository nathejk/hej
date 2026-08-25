# 078 — Backfill/replay verification against real event data

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 006 §9. Prove the projection converges from a real stream: every registered
participant across all four populations resolves by their registered number, crew
land on the right function, and no unmapped-slug fallbacks remain in the final
pre-event data.

Needs a database and a broker, so it cannot be done from unit tests. This is also
where the §11 questions get their real answers: how many phone collisions actually
exist, and whether the slug map covers the organizers' real section names.

## Acceptance Criteria

- [ ] Projection rebuilt from an empty volume against a real stream
- [ ] Row counts per app role sanity-checked against expectations
- [ ] Zero unmapped section slugs, or the map extended until there are
- [ ] Phone collisions counted and the number recorded in PRD 006 §11 Q1
- [ ] Login verified end to end for one person per app role

## Progress Log

- 2026-08-25 — Task created from PRD 006.
