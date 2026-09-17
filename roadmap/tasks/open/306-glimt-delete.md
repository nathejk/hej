# 306 — DELETE /api/glimt/:id — owner only, purges media

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §6, §8. The author can delete their own glimt. **Only** the author — not another
member of the same hold, and **not the Team section**. Moderators hide (task 308); authors
destroy.

Ownership is the `authorPersonId`, not the session, so deletion still works after a profile
switch (PRD 012) and a glimt never becomes unowned.

Publishes `NATHEJK.<year>.glimt.<glimtId>.deleted`; the projection tombstones the row and the
blobs are purged. It must disappear from every cached feed on the next refresh — which is why
the client store replaces rather than merges (task 313).

## Acceptance Criteria

- [ ] `DELETE /api/glimt/:glimtId` publishes the deleted event for the owner
- [ ] Test: a different member of the same hold gets 403
- [ ] Test: a Team-section moderator gets 403 (they hide, not delete)
- [ ] Blobs are purged; media requests for the deleted glimt 404 afterwards
- [ ] Deleting twice is not an error (idempotent)
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
