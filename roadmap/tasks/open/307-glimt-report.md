# 307 — POST /api/glimt/:id/report — synchronous hide from public

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §0, §6. The public scope publishes immediately with no approval queue, so **reporting
is the safety mechanism** and it must not wait on a human.

Any authenticated member can report any glimt they can see. The report **hides it from the
public scope synchronously**, before anyone reviews it. Hiding is cheap and reversible;
leaving something up is not.

Publishes `NATHEJK.<year>.glimt.<glimtId>.reported`. The projection records the report and
increments `reportCount`; `hiddenAt` is set in the same fold so there is no window where a
reported glimt is still public.

Reports must work **without knowing who posted** — the reporter never sees the author, because
nobody outside moderation does (PRD 019 §6).

## Acceptance Criteria

- [ ] `POST /api/glimt/:glimtId/report` accepts an optional reason, publishes the event
- [ ] The glimt is hidden from the public scope with no human step
- [ ] Test: after a report, the public feed query excludes it
- [ ] Test: after a report, it is still visible to its author and to moderators
- [ ] Reporting the same glimt twice does not double-hide or error
- [ ] Rate limited per member so it cannot be used to spam
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
