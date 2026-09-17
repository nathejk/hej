# 302 — Freeze hold attribution; project authorPersonId out

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 019 §6, §8. A glimt is **owned by a person and attributed to their hold**. The person is
never disclosed.

Two halves:

1. **Freeze at creation.** `teamNumber`, `teamName` and `authorGroup` are captured when the
   glimt is created, not resolved at read time, so a glimt keeps saying what it said even if
   a hold is renamed or a member moves. Crew has no hold number — section name + "Crew".
2. **Project the person out.** `authorPersonId` is an ownership and moderation column, not a
   display column. It authorizes `DELETE` and answers a report; it must not appear in any
   response except the Team-section moderation queue. Do this in the **response type**, not
   by trusting each handler to omit it — same discipline `.rules` demands for `phoneParent`.

This includes the author's own feed: the card reads "Din patrulje" plus a *Slet* action, so
ownership is visible without a name ever being on screen.

## Acceptance Criteria

- [x] Creation resolves the author's hold from `person` and stores number, name and group on the glimt row
- [x] A response struct exists that structurally cannot carry `authorPersonId` or a personal name
- [x] Test: renaming a hold does not change an existing glimt's attribution
- [x] Test: the feed payload for the author's own glimt contains no `personId`, no name, no phone
- [x] Test: a crew author gets section name + "Crew" and no hold number
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 13:55 — Picked up. Task 301 already froze the *storage* side (the `glimt` row carries
  `authorGroup`/`teamNumber`/`teamName` and the create fold writes them). What was missing is the
  read side: the derivation at creation, and response types that cannot leak an author.
- 2026-09-17 14:05 — `attributionFor(person, role)` written. Crew branch returns the **section
  name and no number**, which is not missing data — they have a section rather than a numbered
  hold. An unrecognised role returns empty everything so the create handler refuses the post
  rather than guessing a group, since the group is what decides who a `group`-scoped glimt
  reaches.
- 2026-09-17 14:15 — Response types. The enforcement is **structural, not disciplinary**:
  `glimtResponse` has no field capable of holding an author, so a handler cannot leak one by
  forgetting to strip it, and anyone who wants to add one has to add a field and justify it.
- 2026-09-17 14:20 — Decision: **the author's own view is not an exception.** `Own bool` instead
  of sending a name or id for the client to compare. Sending the author's identity "only to
  themselves" would put it in the payload most likely to be persisted to disk, which is exactly
  where we promised it would not be. "Din patrulje" plus a delete action conveys ownership
  without it.
- 2026-09-17 14:30 — Second decision, and the one that closes a real hole: **blob refs are not in
  any response.** Media is addressed as `/api/glimt/{id}/media/{ordinal}`. A content hash in the
  payload would be a bearer capability — forwardable, and impossible to revoke — which is the
  precise mechanism by which a group-scoped photo becomes public. Ordinals are meaningless
  without the glimt id, and the glimt id is access-controlled. Added `HasThumb` so the grid can
  tell "ask for a thumbnail" from "fall back to the full item" without either 404ing or pulling
  full-size media for every tile.
- 2026-09-17 14:35 — `publicGlimtResponse` is a **separate type**, not `glimtResponse` with fields
  omitted. `Own` and `Hidden` are meaningless with no caller, but the real reason is that the
  public payload is the one somebody will eventually have to certify: a distinct struct can be
  read top to bottom, a shared struct with conditional population cannot.
- 2026-09-17 14:40 — `moderationGlimtResponse` is the single disclosure in the feature, in its own
  type so "which responses disclose the author?" is answerable by grep — one type, one handler,
  one route, all requiring the Team-section assignment.
- 2026-09-17 14:50 — ✅ Tests assert against the **serialised JSON**, not the structs. A struct
  test would pass while an embedded type leaked, and JSON is what actually leaves the building.
  The forbidden list covers the author id, any author-shaped key, phone, portrait, and all three
  blob refs.
- 2026-09-17 14:55 — ✅ All criteria met. `gofmt` clean, `go build ./...`, full `go test ./...`
  green. Moving to done.

### Note for task 304 and task 308

`attributionFor` returning an empty group is the signal to **refuse the post** — do not default it
to `crew`, which would put an unclassifiable author's glimt in front of the crew group. And when
task 308 builds the moderation queue, `moderationGlimtResponse` is already the type to use;
resolving `AuthorName` needs a `person` lookup, since a bare id is not something a human moderating
at 03:00 can act on.
