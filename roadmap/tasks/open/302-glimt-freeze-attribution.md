# 302 — Freeze hold attribution; project authorPersonId out

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] Creation resolves the author's hold from `person` and stores number, name and group on the glimt row
- [ ] A response struct exists that structurally cannot carry `authorPersonId` or a personal name
- [ ] Test: renaming a hold does not change an existing glimt's attribution
- [ ] Test: the feed payload for the author's own glimt contains no `personId`, no name, no phone
- [ ] Test: a crew author gets section name + "Crew" and no hold number
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
