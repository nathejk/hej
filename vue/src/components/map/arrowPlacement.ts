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
  /** The next posts, already in route order and already capped. */
  targets: Checkpoint[]
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
 * Returns an empty list rather than throwing for every "we cannot know" case. There are four of them and each
 * is a normal state, not an error:
 *
 *   - **No position.** A bearing needs an origin. The permission card already on screen is the explanation,
 *     and an arrow pointing from a guess would be worse than none.
 *   - **No map yet.** The overlay can mount before Leaflet has a container.
 *   - **The post is already on screen.** Its marker is right there; an arrow pointing at something visible is
 *     noise. Checked with a margin so an arrow does not flicker as a marker grazes the edge.
 *   - **The origin is outside the viewport.** The patrol has panned away from themselves, and there is no
 *     honest edge crossing to compute.
 */
export function computeArrows(input: ArrowInput): Arrow[] {
  const here = input.position
  if (!here) return []

  const size = input.viewportSize()
  if (!size) return []

  const origin = input.project(here.lat, here.lng)
  if (!origin) return []

  const insets = input.insets ?? NO_INSETS
  const out: Arrow[] = []

  for (const cp of input.targets) {
    const target = input.project(cp.lat, cp.lng)
    if (!target) continue
    if (isInside(target, size.width, size.height, ARROW_RADIUS)) continue

    const edge = edgeIntersection(
      origin,
      target,
      size.width,
      size.height,
      ARROW_RADIUS + EDGE_MARGIN,
    )
    if (!edge) continue

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
