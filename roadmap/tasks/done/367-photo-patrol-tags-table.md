# 367 — `photo_patrol`: a photograph is attributed to a patrol, never to a person

**Status:** done
**Priority:** medium
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-22
**Completed:** 2026-09-23

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

**The curator read landed with task 366** as `CuratorQueries.Tags`, returning the id and the number. The
patrol's *name* is deliberately not joined: resolving a number to a name already exists in
`publicpatrol.ByNumber`, and duplicating that join would mean two places decide what a patrol is called — so
the handler resolves it for display, which also keeps this projection out of `public_patrol` entirely.

**This task closed the two assertions** (`photo/tag_test.go`):

`TestATagSurvivesARenumbering` is the one that matters. Because `teamNumber` is not unique per year and
numbers are handed out late, a tag keyed on the number would silently start pointing at a different patrol.
The test pins all three halves of the defence: the insert keys on `teamId`, the update clause refreshes
`teamNumber` but never `teamId`, and the untag matches by id and does not mention the number at all. Once
tags become public (§11 Q1) the failure this prevents is a photograph on the wrong family's page.

`TestNoPublicReadTouchesTheTags` asserts that `querier.go` — the file holding the public interface — does not
know the table's name, and that the public photo type has no tag field, not even a count. "Not yet" is the
kind of constraint that decays into "why not", so it is a test rather than a comment. The complementary half,
keeping `CuratorQueries.Tags` out of public handlers, is `cmd/api/curatorboundary_test.go` from task 366.

Also added `TestUntaggingIsNarrowedToOnePatrol`, which was not asked for: the untag must be scoped by all
three key parts, because dropping the photo id clears that patrol's tag on *every* photograph and dropping
the team id clears *every* tag on that photograph. A group shot with two patrols in it is ordinary, and
correcting one attribution must not clear both.

## Acceptance Criteria

- [x] `photo_patrol` stores `(photoId, teamId)` as its key, plus `year` and the `teamNumber` as it was
      resolved, with a comment recording why both are kept
- [x] `patroltagged` / `patroluntagged` fold idempotently; re-tagging is a no-op, not a duplicate row
- [x] A test proves a tag survives a renumbering: the same `teamId` still resolves after the number
      changes hands
- [x] The table and its event types have no member, name, phone or email field, and no place to add one
- [x] No read reachable from a public handler touches `photo_patrol` — asserted by two tests, one either side
      of the projection boundary
- [x] Untagging leaves the photograph and its other tags intact
- [x] The curator read returns a photograph's tags with the patrol's number, and the name is resolved by the
      handler through the one read that already owns that mapping
