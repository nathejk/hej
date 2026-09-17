# 308 — Moderation API for the Team section

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

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

- [x] `GET /api/glimt/moderation` returns all scopes for a Team-section member, 403 otherwise
- [x] Reported-not-reviewed sorts first
- [x] `hide` / `unhide` publish their events and record `hiddenBy`
- [x] Test: a non-Team member gets 403 on all three endpoints
- [x] Test: hide then unhide restores visibility, and the media was never purged
- [x] Test: the moderation response is the only one containing `authorPersonId`
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 20:35 — Picked up. Ordering was already implemented in task 301's `Moderation()` query
  (`(reportCount > 0 AND hiddenAt IS NULL) DESC` first) and has a querier test; checked that
  criterion off against it rather than re-asserting through HTTP.
- 2026-09-17 20:45 — `requireGlimtModerator` written as an explicit guard **inside each handler**
  rather than as middleware. Two reasons: the lookup needs `app`, and — more importantly — anyone
  reading one of these three handlers should see the check in front of them rather than having to
  trust a route registration elsewhere. This queue takes **no visibility filter at all**, so that
  check is the only thing between it and every photograph in the event.
- 2026-09-17 20:50 — The gate runs **before the id is parsed and before any read**. A test asserts
  `moderationCalls == 0` for a refused caller, so the query genuinely never happens — not merely
  that its result was discarded.
- 2026-09-17 20:55 — `hide` and `unhide` share one implementation. Everything that can go wrong —
  the gate, the existence check, the reason length, the publish failure — is identical, and two
  copies would be two places for the gate to be forgotten.
- 2026-09-17 21:00 — Existence is confirmed **before publishing**, so a typo'd id is a 404 rather
  than an event on the log about nothing — which a replay would later apply to a glimt that happens
  to be created with that id.
- 2026-09-17 21:05 — `AuthorName` is resolved for the queue, because a bare person id is not
  something a human moderating at 03:00 can act on. Best effort: a missing person row leaves it
  empty rather than failing the queue, since an unattributable glimt is exactly the sort somebody
  needs to look at.
- 2026-09-17 21:10 — `Cache-Control: no-store` on the queue. A stale one would have a moderator
  reviewing something already handled — or worse, believing a reported glimt is still up.
- 2026-09-17 21:15 — ✅ The revocation test is the one I most wanted: the **same authenticated
  session**, 200 then 403, with only the person's `sectionSlug` changed in between. That is what a
  session claim could never deliver, and it is now asserted end to end through HTTP rather than only
  at the predicate (task 300).
- 2026-09-17 21:20 — ✅ All criteria met. `gofmt` clean, `go build ./...`, full `go test ./...`
  green. Moving to done.

### Notes for the frontend (task 309) and for task 321

- The client gate on `/glimt/moderation` is **convenience only** — this handler is the control. Do
  not let the view's presence or absence become the security story.
- `moderationGlimtResponse` embeds `glimtResponse`, so it inherits the no-blob-ref property: the
  moderation view fetches media through the same `/items/{id}/media/{ordinal}` route, which the
  moderator override lets through.
- Task 321 owes participants the disclosure this file makes true: the Team section can see **every**
  glimt at every scope, so "Min gruppe" is not "only my gruppe".
