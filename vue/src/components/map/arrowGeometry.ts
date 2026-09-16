// Pointing a patrol at the next post (PRD 016).
//
// The map's one advantage over the paper in their hand is that it knows where they are. These functions turn
// that into an arrow at the edge of the screen: which way, and how far.
//
// # Straight lines, deliberately
//
// Bearing and great-circle distance, not routing (PRD 016 §4). A patrol walks tracks and firebreaks that no
// road graph knows about, so a routed distance would be confidently wrong in a way a straight line is not:
// "3,4 km that way" is honest about being a straight line, while "4,1 km, 52 minutes" implies a path.
//
// # Pure, and separate from Leaflet
//
// Everything here is arithmetic over numbers. It is the part that can be wrong in ways nobody notices —
// a bearing off by 90°, a distance out by a factor of the earth's radius — so it is tested in node rather
// than verified by squinting at a map.

/** A point on the ground. */
export interface LatLng {
  lat: number
  lng: number
}

/** A point in the map container's pixel space, with (0,0) at the top-left. */
export interface Point {
  x: number
  y: number
}

const EARTH_RADIUS_M = 6371008.8

const toRad = (deg: number) => (deg * Math.PI) / 180
const toDeg = (rad: number) => (rad * 180) / Math.PI

/**
 * Great-circle distance in metres.
 *
 * The haversine formula. Accurate to a fraction of a percent at the scale of one event — far better than the
 * GPS fix it is measured from, and it costs a handful of trig calls per arrow per frame.
 */
export function distanceMetres(from: LatLng, to: LatLng): number {
  const dLat = toRad(to.lat - from.lat)
  const dLng = toRad(to.lng - from.lng)
  const lat1 = toRad(from.lat)
  const lat2 = toRad(to.lat)

  const a =
    Math.sin(dLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) ** 2
  return 2 * EARTH_RADIUS_M * Math.asin(Math.min(1, Math.sqrt(a)))
}

/**
 * The angle, clockwise from straight up, that points from one screen point at another.
 *
 * This is what an edge arrow's chevron rotates by so it points at the checkpoint's on-screen location
 * (task 278). Unlike `bearingDegrees` — a fact about the ground that never moves — this is a fact about the
 * *screen*, so it changes on every pan and zoom, which is the point: the arrow must keep aiming at where the
 * post actually is as the map slides beneath it.
 *
 * Clockwise from up, because the chevron graphic points up at rotation 0. `atan2(dx, -dy)`: rotating "up"
 * (0, -1) clockwise by θ gives (sin θ, -cos θ), so a target at (dx, dy) is reached at θ = atan2(dx, -dy).
 *
 * On the north-up map this app uses, up is north, so the result doubles as a compass bearing — which is why
 * the accessible label can be derived from the same number and never disagree with the chevron.
 *
 * Two coincident points have no direction; returns 0 (up) rather than a NaN that would blank the transform.
 */
export function screenBearingDegrees(from: Point, to: Point): number {
  const dx = to.x - from.x
  const dy = to.y - from.y
  if (dx === 0 && dy === 0) return 0
  return (toDeg(Math.atan2(dx, -dy)) + 360) % 360
}

/**
 * The eight-point compass name for a bearing, in Danish.
 *
 * For the accessible label, where "mod nordøst" is usable and "mod 43 grader" is not. Eight points rather
 * than sixteen: nobody navigates a forest at night by "øst-nordøst".
 */
export function compassDanish(bearing: number): string {
  const names = ['nord', 'nordøst', 'øst', 'sydøst', 'syd', 'sydvest', 'vest', 'nordvest']
  const index = Math.round((((bearing % 360) + 360) % 360) / 45) % 8
  return names[index]
}

/**
 * A distance as a patrol should read it, in Danish.
 *
 * Metres below a kilometre, rounded to 10 m — a walking patrol cannot act on single metres, and a number
 * that jumps every step reads as noise. One decimal above it, with a Danish comma.
 */
export function formatDistance(metres: number): string {
  if (metres < 1000) {
    return `${Math.round(metres / 10) * 10} m`
  }
  const km = metres / 1000
  // toFixed(1) then swap the separator: Intl would do it, but constructing a formatter per arrow per frame
  // is exactly the kind of cost that stutters a pan on an old iPad.
  return `${km.toFixed(1).replace('.', ',')} km`
}

/**
 * Where a target's direction crosses the edge of a rectangle, given a centre to point from.
 *
 * # Why the arrow is placed geometrically rather than at a fixed spot per side
 *
 * An arrow pinned to the middle of the nearest edge tells a patrol which edge, not which way. Intersecting
 * the actual line from their position to the post means the arrow slides along the edge as they turn or pan,
 * which is what makes it read as a direction rather than a label.
 *
 * `inset` keeps the arrow fully inside the viewport — it is placed at the arrow's centre, so half its size
 * plus a margin.
 *
 * Returns null when the origin is outside the rectangle: there is no sensible edge crossing then, and the
 * caller has nothing useful to draw.
 */
export function edgeIntersection(
  origin: Point,
  target: Point,
  width: number,
  height: number,
  inset: number,
): Point | null {
  const left = inset
  const right = width - inset
  const top = inset
  const bottom = height - inset

  if (width <= inset * 2 || height <= inset * 2) return null
  if (origin.x < left || origin.x > right || origin.y < top || origin.y > bottom) return null

  const dx = target.x - origin.x
  const dy = target.y - origin.y
  if (dx === 0 && dy === 0) return null

  // How far along the ray we can travel before leaving the box, per axis. The smaller of the two is the
  // crossing.
  let t = Number.POSITIVE_INFINITY
  if (dx > 0) t = Math.min(t, (right - origin.x) / dx)
  if (dx < 0) t = Math.min(t, (left - origin.x) / dx)
  if (dy > 0) t = Math.min(t, (bottom - origin.y) / dy)
  if (dy < 0) t = Math.min(t, (top - origin.y) / dy)

  if (!Number.isFinite(t) || t < 0) return null

  return { x: origin.x + dx * t, y: origin.y + dy * t }
}

/** Whether a point lies inside the rectangle, with a margin. */
export function isInside(p: Point, width: number, height: number, margin = 0): boolean {
  return (
    p.x >= margin && p.x <= width - margin && p.y >= margin && p.y <= height - margin
  )
}
