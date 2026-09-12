import { ref } from 'vue'

import { setMovingSource } from '@/dev/devGeolocation'

// Simulated movement (PRD 014, task 213).
//
// A static coordinate exercises none of what the map actually does: recentring, the track log's
// distance and time filters, tile caching as the view pans. So this walks a route.
//
// # The route is hand-picked, and not the event area
//
// Two constraints, both deliberate:
//
// 1. **Not derived from real track data.** That would be a participant's \u2014 in practice a minor's
//    \u2014 actual movement history in committed source, which `.rules`' privacy posture rules out.
//    (PRD 014 open question 2, settled here.)
// 2. **Not the event area.** `@/config/map` records that the area "is not fully known to
//    participants, so we deliberately do not reveal it". A dev fixture must not be the thing that
//    commits it to the repository. So this is a neutral loop anchored on the same public
//    `FALLBACK_CENTER` that config already ships.
//
// # Interpolated, not vertex-to-vertex
//
// `track.store` filters samples on distance and time deltas, so a marker that teleports between
// vertices exercises different code than one that moves. Interpolation is the point, not polish.

/** A short loop on Sjælland, in [lat, lng]. Roughly 1.5 km end to end. */
const ROUTE: [number, number][] = [
  [55.6, 11.85],
  [55.6035, 11.8525],
  [55.6062, 11.8588],
  [55.6041, 11.8651],
  [55.5998, 11.8637],
  [55.5976, 11.8579],
  [55.6, 11.85],
]

/** Walking pace. The app is used by people on foot, so this is the speed that matters. */
const DEFAULT_SPEED_MPS = 1.4

// Metres per degree, near enough at this latitude. A real projection is not warranted: this feeds
// a simulated walk, and being 1% out changes nothing about what is under test.
const M_PER_DEG_LAT = 111_320
const M_PER_DEG_LNG = 111_320 * Math.cos((55.6 * Math.PI) / 180)

function segmentMetres(a: [number, number], b: [number, number]): number {
  const dLat = (b[0] - a[0]) * M_PER_DEG_LAT
  const dLng = (b[1] - a[1]) * M_PER_DEG_LNG
  return Math.hypot(dLat, dLng)
}

const LEGS = ROUTE.slice(0, -1).map((from, i) => ({
  from,
  to: ROUTE[i + 1],
  metres: segmentMetres(from, ROUTE[i + 1]),
}))

const TOTAL_METRES = LEGS.reduce((sum, leg) => sum + leg.metres, 0)

const playing = ref(false)
const speed = ref(DEFAULT_SPEED_MPS)
// Distance travelled along the route, in metres. Kept rather than a timestamp so pausing does not
// silently advance the walk.
const travelled = ref(0)
let lastTick = 0

export const playbackPlaying = playing
export const playbackSpeed = speed
export const playbackProgress = travelled

/** Where the walk currently is, or `null` when it has never started. */
function currentCoord(): { lat: number; lng: number } | null {
  advance()
  let remaining = travelled.value
  for (const leg of LEGS) {
    if (remaining <= leg.metres) {
      const t = leg.metres === 0 ? 0 : remaining / leg.metres
      return {
        lat: leg.from[0] + (leg.to[0] - leg.from[0]) * t,
        lng: leg.from[1] + (leg.to[1] - leg.from[1]) * t,
      }
    }
    remaining -= leg.metres
  }
  // Past the end: sit on the last vertex. The route is a loop, so this is also the start \u2014 which
  // is why it does not need to emit a discontinuity to get back there.
  const last = ROUTE[ROUTE.length - 1]
  return { lat: last[0], lng: last[1] }
}

// Advances by wall-clock time since the last read, so the pace is right regardless of how often
// the position is sampled. A per-tick increment would make the walk's speed depend on the watch
// interval, which is the sort of coupling that makes a simulation lie.
function advance() {
  const now = Date.now()
  if (!playing.value) {
    lastTick = now
    return
  }
  if (lastTick === 0) {
    lastTick = now
    return
  }
  const elapsed = (now - lastTick) / 1000
  lastTick = now
  travelled.value += elapsed * speed.value
  if (travelled.value > TOTAL_METRES) {
    // Loop rather than stop. The route returns to its own start, so wrapping produces no jump:
    // a discontinuity here would look exactly like a GPS glitch and would be recorded as one.
    travelled.value -= TOTAL_METRES
  }
}

export function startPlayback() {
  lastTick = Date.now()
  playing.value = true
}

export function pausePlayback() {
  advance()
  playing.value = false
}

export function resetPlayback() {
  playing.value = false
  travelled.value = 0
  lastTick = 0
}

/** Registers the walk as the fake source's position. Called once from `@/dev/bootstrap`. */
export function initPlayback() {
  // Returns null while stopped at the very beginning, so the fixed coordinate is used instead and
  // the fake position works without playback ever being started.
  setMovingSource(() => (playing.value || travelled.value > 0 ? currentCoord() : null))
}

/** Total route length, for the panel's readout. */
export const ROUTE_METRES = TOTAL_METRES
