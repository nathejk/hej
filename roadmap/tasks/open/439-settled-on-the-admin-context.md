# 439 — ctx.settled(ids): one place that waits for the projection

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

From PRD 024 §8, and noted as the right shape by both task 437 and task 438.

Every write in the admin tool publishes an event and then calls `ctx.reloadSheet()` in the same tick,
reading a projection that is folded downstream. The uploader now waits properly (task 438: poll
`GET /api/admin/photos?ids=…` until the ids come back), and PRD 024 adds a third caller — a resort is a
publish like any other, and its reload has the same race.

Eight files have the bug today: `albumaction`, `deleteaction`, `patrolaction`, `positionaction`,
`captionaction`, `creditaction`, `albumorder`, `vieweredit`. They are less likely to be *noticed* than
the upload was — one row folded rather than three hundred, and the visible change is an attribute on a
cell that is already there rather than cells that are absent — but the shape is identical.

Do this **before** PRD 024's apply-the-sort tasks, so the third caller is a call and not a third copy.

## Acceptance Criteria

- [ ] `ctx.settled(ids)` on the admin context: resolves when the library can name every id, or when a
      bounded wait gives up; reports which happened
- [ ] The uploader is moved onto it and `waitForPhotos` leaves `upload.js`
- [ ] The uploader's behaviour and its guard are unchanged in substance
- [ ] `TestEveryAdminContextMemberIsProvided` covers the new member (it will, by construction)
- [ ] Callers that reload after a write but have no ids to wait for are left alone, not given a fake wait

## Progress Log

- 2026-09-28 — Task created from PRD 024 §8.
