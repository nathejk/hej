import {
  bearingDegrees,
  compassDanish,
  distanceMetres,
  edgeIntersection,
  formatDistance,
  isInside,
  type Point,
} from '@/components/map/arrowGeometry'
import type { KeepOutZone } from '@/config/map'
import type { Checkpoint } from '@/stores/checkpoints.store'
import type { Coords } from '@/stores/location.store'

// Re-exported so the overlay component and its spec have one import for the arrow types.
export type { KeepOutZone }

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

/** Extra clearance between the arrow and the viewport edge, so it hugs the edge without sitting half off. */
const EDGE_MARGIN = 8

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
   * The posts to arrow: every revealed post in the line the patrol is heading for.
   *
   * A *line* holds several posts — an A and a B — and the patrol heads for the line, choosing which post when
   * they arrive. So each of them gets its own arrow and the choice stays with the people walking (task 275).
   * Which line is next is decided by the BFF, which is the only side that knows route order and whether the
   * patrol has started.
   */
  targets: Checkpoint[]
  /** The patrol's own position, or null. */
  position: Coords | null
  /** Projects a ground position into container pixels; null before the map exists. */
  project: (lat: number, lng: number) => Point | null
  /** The map container's pixel size; null before the map exists. */
  viewportSize: () => { width: number; height: number } | null
  /**
   * Rectangles the arrow must not sit on, because a floating control does (task 264).
   *
   * The arrow slides *along its edge* to clear these rather than being pushed inward — see slideClear. An
   * empty list means every edge is free, which is the norm; the controls occupy only two corners.
   */
  keepOut?: KeepOutZone[]
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
 * One arrow per post in the next line: the patrol chooses which of a line's posts to walk to, so the app shows
 * them all rather than nominating one.
 */
export function computeArrows(input: ArrowInput): Arrow[] {
  const here = input.position
  if (!here) return []

  const size = input.viewportSize()
  if (!size) return []

  // Fixed in container coordinates, and always inside the box — see the note above.
  const centre: Point = { x: size.width / 2, y: size.height / 2 }

  const keepOut = input.keepOut ?? []
  const out: Arrow[] = []

  for (const cp of input.targets) {
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
      ...slideClear(edge, size, keepOut),
      bearing,
      distance,
      label: `${cp.name}, ${distance} mod ${compassDanish(bearing)}`,
    })
  }
  return out
}

/**
 * Slide a point *along the edge it sits on* until it clears the keep-out zones.
 *
 * This replaces the band-clamp that shipped in task 264 and caused the bug in task 276. The band pushed every
 * arrow inward by the height of the controls, so an arrow leaving the top edge on the *left* — where nothing
 * floats — hung 112 px in mid-air instead of at the edge. Sliding keeps the arrow hugging the edge and simply
 * moves it past the control: an arrow that would sit under the top-right stack slides left along the top edge,
 * or down along the right edge, to just clear it.
 *
 * The edge the point is on is read from which coordinate is pinned to the inset. A point on a horizontal edge
 * slides in x; on a vertical edge, in y. It moves to the nearer clear side of the zone, so the arrow stays as
 * close as it can to where the post actually is.
 *
 * A zone that spans the whole edge would leave nowhere to slide to; the final clamp keeps the point on screen
 * rather than off it, which is the only case where an arrow can still end up touching a control. The controls
 * are corner-sized, so it does not arise in practice.
 */
function slideClear(
  p: Point,
  size: { width: number; height: number },
  zones: KeepOutZone[],
): Point {
  const inset = ARROW_RADIUS + EDGE_MARGIN
  const near = 0.5
  const onHorizontalEdge =
    Math.abs(p.y - inset) < near || Math.abs(p.y - (size.height - inset)) < near

  let { x, y } = p
  for (const z of zones) {
    const inX = x >= z.x - ARROW_RADIUS && x <= z.x + z.width + ARROW_RADIUS
    const inY = y >= z.y - ARROW_RADIUS && y <= z.y + z.height + ARROW_RADIUS
    if (!inX || !inY) continue

    if (onHorizontalEdge) {
      const left = z.x - ARROW_RADIUS
      const right = z.x + z.width + ARROW_RADIUS
      x = Math.abs(x - left) <= Math.abs(x - right) ? left : right
    } else {
      const up = z.y - ARROW_RADIUS
      const down = z.y + z.height + ARROW_RADIUS
      y = Math.abs(y - up) <= Math.abs(y - down) ? up : down
    }
  }

  return {
    x: clamp(x, inset, size.width - inset),
    y: clamp(y, inset, size.height - inset),
  }
}

function clamp(value: number, min: number, max: number): number {
  if (min > max) return (min + max) / 2
  return Math.min(Math.max(value, min), max)
}
