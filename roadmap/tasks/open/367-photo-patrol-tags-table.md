# 367 — `photo_patrol`: a photograph is attributed to a patrol, never to a person

**Status:** open
**Priority:** medium
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

A new join table `photo_patrol` in the `photo` projection, plus the two events
`patroltagged` / `patroluntagged` on `NATHEJK.<year>.photo.<photoId>.*` (PRD 022 §8.7) and their fold.
A photograph may carry more than one tag, because two patrols in one frame is ordinary (PRD 022 §6).

**The tag stores the resolved `teamId` as well as the number, and this is a hazard rather than a
nicety.** PRD 022 §8.6: `teamNumber` is **not unique per year** — the index is deliberately non-unique
and `publicpatrol.Queries.ByNumber` does `ORDER BY teamId LIMIT 1`. A tag that stored only the number
would silently point at a different patrol after a renumbering, on a family's page, which is precisely
the failure mode §11 Q1 defers the public surfacing to avoid.

The table names **a patrol, never a person**. There is no field for a member and none may be added;
PRD 022 §4 repeats this specifically because a tagging UI is exactly where somebody reaches for a
name, and `.rules` and PRD 011 §4 both already require it.

**No public read may be written against `photo_patrol` in v1.** PRD 022 §11 Q1 resolved that a tag
will eventually surface a photograph on that patrol's public page — but deliberately not yet. In v1 a
tag is curator metadata with no public effect, so the tagging can be used in anger, checked for
accuracy and corrected before a mistag can put a photograph on the wrong family's page. Surfacing it
is its own task and its own review.

## Progress

**The schema, the two events and the fold landed early, with task 363.** Not scope creep: the `photo`
consumer subscribes to every verb the entity will ever publish, following the rule the album consumer
established — a projection that ignores an event it was not yet taught about would silently keep showing
a photograph somebody took down. A subscription to a verb whose table does not exist fails on the first
message, so `photo_patrol` had to exist at the same moment the subscription did.

Already done and tested in `go/nathejk/table/photo/`: the table with `PRIMARY KEY (year, photoId,
teamId)` and both indexes; `PatrolTagged` / `PatrolUntagged` with `TeamID` **and** `Number`; the
idempotent tag fold (re-tagging converges and supersedes an earlier untag); the refusal of a tag with no
`teamId`; and the soft-delete untag.

**What is left** is the reads and the two assertions:

## Acceptance Criteria

- [x] `photo_patrol` stores `(photoId, teamId)` as its key, plus `year` and the `teamNumber` as it was
      resolved, with a comment recording why both are kept
- [x] `patroltagged` / `patroluntagged` fold idempotently; re-tagging is a no-op, not a duplicate row
- [ ] A test proves a tag survives a renumbering: the same `teamId` still resolves after the number
      changes hands
- [x] The table and its event types have no member, name, phone or email field, and no place to add one
- [ ] No read reachable from a public handler touches `photo_patrol` — asserted by a test, not by
      inspection
- [x] Untagging leaves the photograph and its other tags intact
- [ ] The curator read returns a photograph's tags with the patrol's number and name for display
