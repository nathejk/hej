# 308 — Moderation API for the Team section

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §0, §6, §8. The Team section (slug `team`) can see everything and hide anything.

- `GET /api/glimt/moderation` — every glimt at every scope, **reported-and-not-yet-reviewed
  first**. This is the one response that carries `authorPersonId`, because moderation cannot
  work against an anonymous author (task 302 projects it out everywhere else).
- `POST /api/glimt/:glimtId/hide` and `/unhide` — reversible, records `hiddenBy`, publishes
  `.hidden` / `.unhidden`.

Authority comes from task 300's per-request section lookup, never from a session claim.

**Hiding is not deleting.** Hidden glimt stay visible to moderators and to their author, and
the media survives so an unhide is possible. Only the author's `DELETE` (task 306) destroys
media.

## Acceptance Criteria

- [ ] `GET /api/glimt/moderation` returns all scopes for a Team-section member, 403 otherwise
- [ ] Reported-not-reviewed sorts first
- [ ] `hide` / `unhide` publish their events and record `hiddenBy`
- [ ] Test: a non-Team member gets 403 on all three endpoints
- [ ] Test: hide then unhide restores visibility, and the media was never purged
- [ ] Test: the moderation response is the only one containing `authorPersonId`
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
