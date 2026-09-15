# 273 — Arrows point at unvisited checkgroups, not unvisited checkpoints

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

Correction to task 263, raised by the product owner.

The edge arrows currently skip a checkpoint the patrol has already **scanned** and point at the next
unvisited *checkpoint* in route order. That is the wrong unit. The unit of progress in this domain is the
**checkgroup**:

- HQ's own model is explicit about it — `checkgroupteams.go` computes "a started team's standing at one
  checkgroup", with `onTime` / `late` / `missing` per *group*, not per post.
- Reveal rule 3 follows the same logic: scanning any checkpoint reveals its **whole** checkgroup, which
  only makes sense if the posts in a group are alternatives along one leg rather than a list of places a
  patrol must each visit.

So a patrol that has scanned at Post 4A has finished that leg, and an arrow towards Post 4B — the
alternative post in the same group — sends them somewhere they have no reason to go. Worse, it consumes
one of the three arrow slots that should be showing the legs ahead.

Two changes:

1. **A visited checkgroup is done.** Every checkpoint in a group containing any visited checkpoint is
   excluded from the arrows.
2. **One arrow per group.** The posts in an unvisited group are alternatives, so the arrow points at the
   one the patrol can actually reach — the **nearest** — and the cap of three then means three *legs*
   ahead rather than three posts that might all be one leg.

Markers are unaffected: they still show every revealed post, with the ones actually scanned ticked. The
map should show the ground truth; only the arrows are an instruction.

## Acceptance Criteria

- [x] A checkpoint whose checkgroup has a visited post produces no arrow (test).
- [x] An unvisited group produces exactly one arrow, pointing at its nearest post (test).
- [x] The cap of three applies to groups, so one multi-post group cannot hide the legs behind it (test).
- [x] Markers still show every revealed checkpoint, ticked where scanned (unchanged).
- [x] PRD 016 §6 amended, since it specified this in terms of checkpoints (also §11.12, recording why).
- [x] All four Go gates and the frontend suite clean.

## Progress Log

- 2026-09-15 — Task created from the product owner's correction. Confirmed the domain reading against
  hq's `checkgroupteams.go` before changing anything: a team's standing is recorded per checkgroup, which
  is what makes the posts within one alternatives rather than a checklist.
- 2026-09-15 — `nextCheckpoints.ts` reworked from `nextCheckpoints` (posts) to `nextCheckpointGroups`
  (legs). A group is finished if **any** of its posts was scanned; finished groups are excluded whole.
- 2026-09-15 — Decision: within a leg the arrow points at the **nearest** post, not the first in route
  order. The posts are alternatives, so the patrol needs one of them and the useful answer is the one they
  can walk to — taking the first would be deterministic and would sometimes send them past the closer
  option. Needs the position, so the choice lives in `arrowPlacement` (which has it) rather than in the
  grouping. Tested by listing the far post first, so an implementation that took `[0]` fails.
- 2026-09-15 — Decision: the cap of three now counts **legs**. That is the substantive half of this change
  rather than a detail — with the old per-post cap, a leg with three alternative posts filled the viewport
  and hid every leg behind it, which is the opposite of what the arrows are for.
- 2026-09-15 — Edge case the correction surfaced: a post can be revealed **before its checkgroup is known**,
  because the group arrives on `checkpoint.created` while the name and position arrive on
  `checkpoint.updated` — a catching-up projection can hold one without the other. Such posts are keyed as
  their own single-post leg rather than merged into one nameless group, which would have arrowed one of them
  and silently hidden the rest. Two tests cover it, including that visiting one group-less post does not take
  the others with it.
- 2026-09-15 — **Markers deliberately unchanged.** They still show every revealed post, ticked where
  scanned. The map should show the ground truth — including the alternative post the patrol did not use, which
  is useful for orientation; only the arrows are an *instruction*, and only an instruction has to be about the
  leg.
- 2026-09-15 — PRD 016 §6 amended and the reasoning recorded as §11.12, so the reversal is legible rather
  than looking like the plan all along.
- 2026-09-15 — ✅ 72 map specs (11 grouping, 18 placement); frontend suite 605 tests, type-check and build
  clean; all four Go gates clean — including `staticcheck` this time (task 272).
- 2026-09-15 — Done.
