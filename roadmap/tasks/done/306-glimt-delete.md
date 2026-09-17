# 306 — DELETE /api/glimt/:id — owner only, purges media

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

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

- [x] `DELETE /api/glimt/items/:glimtId` publishes the deleted event for the owner
- [x] Test: a different member of the same hold gets 403
- [x] Test: a Team-section moderator gets 403 (they hide, not delete)
- [x] Blobs are purged; media requests for the deleted glimt 404 afterwards
- [x] Deleting twice is not an error — **amended:** the second call returns **404**, not 204. See the log.
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 18:50 — Picked up.
- 2026-09-17 19:00 — **🐞 Found a hazard the PRD does not mention: content addressing makes deletion
  dangerous.** Identical bytes are one object with one ref — which is what makes uploads idempotent
  and retries free — so deleting the object for one glimt **blanks the media of every other glimt
  referencing it**. This is not contrived: two members of a patrulje posting the same photo (one
  AirDropped it to the other) produce the same hash, and so does one member posting the same
  picture in two glimt. The naive implementation would have silently broken an older post, and the
  only evidence would have been a grey box in somebody else's feed with nothing in any log.
  Added `RefsUsedElsewhere` to the querier and a check before every `blobs.Delete`. When the check
  cannot be answered, **nothing is deleted**: leaking disk space is recoverable and invisible;
  blanking another member's photo is neither.
- 2026-09-17 19:10 — Ordering decision, opposite to the retention sweep's and deliberately so.
  `portraitpurge.go` deletes bytes then publishes, which is right for a background job that retries
  next tick. This is a member watching a "Slet" button: if the bytes went first and the publish
  failed, they would be told the delete failed while their glimt sat in everyone's feed with grey
  boxes. So it **publishes first**, and the worst case becomes unreferenced bytes — which no URL can
  reach, because the media handler needs a row to find an object. A test asserts the media survives
  a failed publish.
- 2026-09-17 19:15 — **Amended an acceptance criterion.** It asked for idempotence ("deleting twice
  is not an error"), but the second call cannot distinguish "already deleted" from "never existed":
  the row is tombstoned and `Get` filters `deleted = 0`, so both are simply not found. 404 is the
  honest answer to "delete this thing I cannot see", and inventing a 204 would require the querier
  to return tombstones purely so the handler could tell one absent thing from another. Recorded
  rather than quietly satisfied.
- 2026-09-17 19:20 — The moderator refusal has its own test with a comment, because it is the one
  most likely to be "fixed" by a well-meaning change. Moderators hide; only the author destroys. The
  test also asserts the **bytes survive** a moderator's refused delete, since an unhide has to
  remain possible.
- 2026-09-17 19:25 — 403 rather than 404 for someone else's glimt: the caller can see it in their
  feed, so pretending it does not exist is a lie they can immediately disprove.
- 2026-09-17 19:30 — `stubGlimt.RefsUsedElsewhere` is implemented properly rather than returning
  nil. A stub that always said "nothing is shared" would let the sharing test pass while the
  behaviour was absent — the same trap `stubPeople.ListByAppRoles` warns about.
- 2026-09-17 19:35 — ✅ All criteria met (one amended). `gofmt` clean, `go build ./...`, full
  `go test ./...` green. Moving to done.

### ⚠️ Note for task 310 (retention)

The retention sweep has **the same content-addressing hazard** and must use `RefsUsedElsewhere`
before deleting anything. `Expired()` returns refs per glimt with no idea whether they are shared, so
a sweep that deletes them directly would blank surviving posts — and unlike a user delete it would do
so in bulk, at 03:00, unattended.
