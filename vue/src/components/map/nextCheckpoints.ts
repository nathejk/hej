import type { Checkpoint } from '@/stores/checkpoints.store'

// Which posts a patrol is heading for (PRD 016).
//
// # "Next" is route order, not distance
//
// The nearest revealed post is not necessarily the next one: a patrol walking a loop passes close to posts it
// has already visited and to posts several legs ahead. Pointing at the nearest would send them backwards.
//
// So "next" is the earliest **not yet visited** post in route order — and route order is decided by the BFF,
// which is the only side that has both halves of it (the checkgroup's position and the post's position within
// it). The list arrives already ordered and this module preserves that order rather than re-deriving it.
//
// # Capped, on purpose
//
// Three arrows at most. The viewport is a phone screen at night; four arrows is a decoration rather than an
// instruction, and the third is already only useful for orientation.

/** How many arrows may be on screen at once. */
export const MAX_ARROWS = 3

/**
 * The next posts for this patrol, in route order.
 *
 * `visitedIds` are the checkpoints the patrol has already scanned. A visited post is skipped even if posts
 * before it are unvisited — a patrol may legitimately reach posts out of order, and continuing to point at
 * something they have already stood at would be worse than saying nothing.
 */
export function nextCheckpoints(
  checkpoints: Checkpoint[],
  visitedIds: Iterable<string>,
  limit = MAX_ARROWS,
): Checkpoint[] {
  const visited = new Set(visitedIds)
  const out: Checkpoint[] = []

  for (const cp of checkpoints) {
    if (visited.has(cp.id)) continue
    out.push(cp)
    if (out.length >= limit) break
  }
  return out
}
