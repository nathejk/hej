import {
  bearingDegrees,
  compassDanish,
  distanceMetres,
  edgeIntersection,
  formatDistance,
  isInside,
  type Point,
} from '@/components/map/arrowGeometry'
import type { Checkpoint } from '@/stores/checkpoints.store'
import type { Coords } from '@/stores/location.store'

// Placing the edge arrows (PRD 016).
//
// # Why this is a pure function and not just the component's computed
//
// Every rule that decides whether an arrow appears — no position, post already on screen, no room in the
// viewport, keep clear of the floating controls — is a decision a patrol will be affected by at 02:00 in a
// forest, and none of it can be verified by looking at a screenshot. Vitest here runs in node with no DOM, so
// the alternative to extracting this is not testing it.
//
// The component keeps the markup; this keeps the judgement.

/** Half the arrow's rendered size, so geometry can keep it fully on screen. */
export const ARROW_RADIUS = 26

/** Extra clearance between the arrow and the viewport edge, so it does not sit half off. */
const EDGE_MARGIN = 8

/** Margins an arrow must not intrude into, because floating controls live there. */
export interface Insets {
  top: number
  right: number
  bottom: number
  left: number
}

export const NO_INSETS: Insets = { top: 0, right: 0, bottom: 0, left: 0 }

export interface Arrow {
  id: string
  name: string
  x: number
  y: number
  /** Degrees clockwise from north, for the chevron's rotation. */
  bearing: number
  distance: string
  /** The accessible label — the non-visual route to the same fact. */
  label: string
}

export interface ArrowInput {
  /**
   * The next legs, in route order and already capped — each entry is one checkgroup's posts.
   *
   * A group's posts are alternatives, so exactly one arrow is drawn per group (see `pickReachable`).
   */
  groups: Checkpoint[][]
  /** The patrol's own position, or null. */
  position: Coords | null
  /** Projects a ground position into container pixels; null before the map exists. */
  project: (lat: number, lng: number) => Point | null
  /** The map container's pixel size; null before the map exists. */
  viewportSize: () => { width: number; height: number } | null
  insets?: Insets
}

/**
 * Which arrows to draw, and where.
 *
 * # Placement is measured from the centre of the viewport, not from the patrol
 *
 * This is the fix for a reported bug, and the reasoning is worth keeping. The first version drew each arrow
 * where the line **from the patrol's own position** to the post crossed the screen edge. That reads well while
 * the map is following them — they are in the middle, so the line leaves the screen exactly where the post is —
 * and it falls apart the moment anybody pans or zooms:
 *
 *   - **Arrows vanished.** A pan or a zoom-out routinely puts the patrol's own position off screen, and a ray
 *     needs an origin *inside* the box to have an edge crossing at all. So every arrow disappeared together,
 *     exactly when the user was looking around for their next post.
 *   - **Arrows jumped.** As the patrol's dot slid across the screen during a drag, the ray pivoted around it,
 *     and the crossing point swung along the edge far faster than the map moved underneath.
 *
 * The viewport centre is always inside the viewport, and in container coordinates it never moves — so an arrow
 * for an off-screen post is always placeable, and it tracks the post smoothly as the map slides beneath it.
 * When the map *is* following the patrol the centre and the patrol coincide, which is the case the original
 * geometry was tuned for, so nothing is lost there.
 *
 * # What stays measured from the patrol
 *
 * The **bearing** and the **distance**: those are facts about the ground, not about the screen. The chevron
 * means "walk this way" and the label says how far, and neither may change because somebody dragged the map.
 *
 * # Why it returns an empty list rather than throwing
 *
 * Three "we cannot know" cases, each a normal state rather than an error:
 *
 *   - **No position.** A bearing needs somewhere to measure from. The permission card already on screen is the
 *     explanation, and an arrow drawn from a guess would be worse than none.
 *   - **No map yet.** The overlay can mount before Leaflet has a container.
 *   - **The post is already on screen.** Its marker is right there; an arrow pointing at something visible is
 *     noise. Checked with a margin so an arrow does not flicker as a marker grazes the edge.
 *
 * One arrow per leg: a checkgroup's posts are alternatives, so only the reachable one is drawn.
 */
export function computeArrows(input: ArrowInput): Arrow[] {
  const here = input.position
  if (!here) return []

  const size = input.viewportSize()
  if (!size) return []

  // Fixed in container coordinates, and always inside the box — see the note above.
  const centre: Point = { x: size.width / 2, y: size.height / 2 }

  const insets = input.insets ?? NO_INSETS
  const out: Arrow[] = []

  for (const group of input.groups) {
    const cp = pickReachable(group, here)
    if (!cp) continue

    const target = input.project(cp.lat, cp.lng)
    if (!target) continue
    if (isInside(target, size.width, size.height, ARROW_RADIUS)) continue

    const edge = edgeIntersection(
      centre,
      target,
      size.width,
      size.height,
      ARROW_RADIUS + EDGE_MARGIN,
    )
    if (!edge) continue

    // From the patrol, not from the centre: this is the direction to walk and the distance to walk it.
    const bearing = bearingDegrees(here, cp)
    const distance = formatDistance(distanceMetres(here, cp))

    out.push({
      id: cp.id,
      name: cp.name,
      ...clampToBand(edge, size, insets),
      bearing,
      distance,
      label: `${cp.name}, ${distance} mod ${compassDanish(bearing)}`,
    })
  }
  return out
}

/**
 * Which post in a leg to point at.
 *
 * The **nearest**, because a checkgroup's posts are alternatives — the patrol needs one of them, so the useful
 * answer is the one they can actually walk to. Pointing at the group's first post in route order would be
 * deterministic and sometimes send them past the closer option for no reason.
 *
 * Straight-line distance, like everything else here: it does not know about the stream they would have to
 * cross, and it does not pretend to (PRD 016 §4).
 */
function pickReachable(group: Checkpoint[], from: Coords): Checkpoint | null {
  let best: Checkpoint | null = null
  let bestMetres = Number.POSITIVE_INFINITY

  for (const cp of group) {
    const metres = distanceMetres(from, cp)
    if (metres < bestMetres) {
      best = cp
      bestMetres = metres
    }
  }
  return best
}

/**
 * Move a point out of the margins the floating controls occupy.
 *
 * **Clamped rather than dropped**, deliberately (task 264): direction is approximate at the edge anyway, so a
 * few pixels of slide costs almost nothing — while losing the arrow loses the only thing on screen telling
 * the patrol which way to walk.
 *
 * When the band is narrower than the arrow — a very short viewport — the arrow is centred in what is left
 * rather than pushed off one side, because clamping twice against contradictory bounds would otherwise pin it
 * outside the screen.
 */
function clampToBand(
  p: Point,
  size: { width: number; height: number },
  insets: Insets,
): Point {
  return {
    x: clamp(p.x, insets.left + ARROW_RADIUS, size.width - insets.right - ARROW_RADIUS),
    y: clamp(p.y, insets.top + ARROW_RADIUS, size.height - insets.bottom - ARROW_RADIUS),
  }
}

function clamp(value: number, min: number, max: number): number {
  if (min > max) return (min + max) / 2
  return Math.min(Math.max(value, min), max)
}
