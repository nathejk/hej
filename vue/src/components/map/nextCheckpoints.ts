import type { Checkpoint } from '@/stores/checkpoints.store'

// Which legs a patrol is heading for (PRD 016, task 273).
//
// # The unit is the checkgroup, not the checkpoint
//
// A checkgroup is one leg of the route, and the posts in it are **alternatives**: a patrol is scanned *at a
// checkgroup*, which is how HQ records progress too ("a started team's standing at one checkgroup" — on time,
// late or missing, per group). It is also why scanning any post reveals its whole group.
//
// So a patrol that has scanned at Post 4A has finished that leg. An arrow towards Post 4B would send them
// somewhere they have no reason to go, and would spend one of three arrow slots that should be showing the
// legs ahead.
//
// # "Next" is route order, not distance
//
// The nearest revealed post is often not the next one: a patrol walking a loop passes close to legs it has
// already done and legs several ahead. Pointing at the nearest would send them backwards. Route order comes
// from the BFF — the only side holding both halves of it (the group's position and the post's position within
// it) — so this module preserves the order it was given rather than re-deriving it.
//
// # Capped, on purpose
//
// Three legs at most. The viewport is a phone screen at night; a fourth arrow is decoration rather than an
// instruction, and the third is already only useful for orientation.

/** How many legs may be arrowed at once. */
export const MAX_ARROW_GROUPS = 3

/**
 * The next unvisited legs, in route order, each as its group's posts.
 *
 * `visitedIds` are the checkpoints the patrol has actually scanned. A group is finished if **any** of its
 * posts was scanned, because they are alternatives.
 *
 * A group is skipped even when earlier groups are unfinished: patrols legitimately do legs out of order — a
 * skitse sends them one way, or they simply walk it their own way — and continuing to point at a leg they have
 * already completed would be worse than silence.
 *
 * Checkpoints with no checkgroup (possible while the projections are catching up, since the group arrives on a
 * different event) are treated as their own single-post group rather than lumped together: they are real posts
 * the patrol may need, and merging them would arrow one and hide the rest.
 */
export function nextCheckpointGroups(
  checkpoints: Checkpoint[],
  visitedIds: Iterable<string>,
  limit = MAX_ARROW_GROUPS,
): Checkpoint[][] {
  const visited = new Set(visitedIds)

  const finishedGroups = new Set<string>()
  for (const cp of checkpoints) {
    if (visited.has(cp.id)) {
      finishedGroups.add(groupKeyFor(cp))
    }
  }

  // Insertion-ordered, so the groups come back in the route order the BFF sent.
  const groups = new Map<string, Checkpoint[]>()
  for (const cp of checkpoints) {
    const key = groupKeyFor(cp)
    if (finishedGroups.has(key)) continue

    const existing = groups.get(key)
    if (existing) {
      existing.push(cp)
    } else {
      groups.set(key, [cp])
    }
  }

  return [...groups.values()].slice(0, limit)
}

// A post with no checkgroup yet stands alone, keyed by its own id. See the note on
// nextCheckpointGroups.
function groupKeyFor(cp: Checkpoint): string {
  return cp.checkgroup === '' ? `cp:${cp.id}` : `cg:${cp.checkgroup}`
}
