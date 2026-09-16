import { describe, expect, it } from 'vitest'

import {
  ARROW_RADIUS,
  computeArrows,
  type ArrowInput,
} from '@/components/map/arrowPlacement'
import { isInside } from '@/components/map/arrowGeometry'
import { arrowKeepOutZones } from '@/config/map'
import type { Checkpoint } from '@/stores/checkpoints.store'

const W = 400
const H = 800

// A notched phone's control zones, which is the case a desktop-tuned constant would get wrong.
const ZONES = arrowKeepOutZones({ width: W, height: H }, { top: 59, bottom: 34 })

function cp(id: string, lat: number, lng: number): Checkpoint {
  return {
    id,
    name: `Post ${id}`,
    checkgroup: 'cg-1',
    sortOrder: 0,
    lat,
    lng,
    openFrom: 0,
    openUntil: 0,
    openDurationMinutes: 0,
  }
}

// A fake projection: a flat, linear mapping from degrees to pixels around a fixed origin. Crude on purpose —
// the real Web Mercator maths is Leaflet's job and is not what these rules are about, and a linear stand-in
// makes each test's geometry obvious from the numbers.
//
// It takes the viewport it is projecting into, because a projection and a container size that disagree put the
// patrol's own position outside the box — which the code correctly refuses to draw arrows from. (My first
// version hardcoded one size and produced exactly that confusing failure.)
//
// `pxPerDegree` stands in for zoom, and it matters: at 2000 px/° an off-screen post is tens of kilometres
// away, while at 200 000 px/° — a patrol zoomed in on themselves, which is the normal case — it is a few
// hundred metres. Tests about distance formatting have to pick the zoom that makes their distance plausible.
function projectionAround(
  centre: { lat: number; lng: number },
  size: { width: number; height: number } = { width: W, height: H },
  pxPerDegree = 2000,
) {
  return (lat: number, lng: number) => ({
    x: size.width / 2 + (lng - centre.lng) * pxPerDegree,
    y: size.height / 2 - (lat - centre.lat) * pxPerDegree,
  })
}

function input(over: Partial<ArrowInput> = {}): ArrowInput {
  const here = { lat: 56, lng: 9, accuracy: 10 }
  return {
    targets: [],
    position: here,
    project: projectionAround(here),
    viewportSize: () => ({ width: W, height: H }),
    ...over,
  }
}

describe('computeArrows', () => {
  // A post beyond the top of the screen: the patrol is walking towards something they cannot see, which is the
  // entire reason this feature exists.
  it('draws an arrow for a post off the top of the screen', () => {
    const got = computeArrows(input({ targets: [cp('a', 56.5, 9)] }))

    expect(got).toHaveLength(1)
    expect(got[0].id).toBe('a')
    // Due north, so the arrow sits at the top edge, horizontally in line with the patrol.
    expect(got[0].y).toBeLessThan(H / 2)
    expect(got[0].x).toBeCloseTo(W / 2, 0)
  })

  // A post already on screen has a marker right there. An arrow pointing at something visible is noise.
  it('draws no arrow for a post already on screen', () => {
    const got = computeArrows(input({ targets: [cp('a', 56.01, 9.01)] }))

    expect(got).toEqual([])
  })

  // A bearing needs an origin. The permission card already on screen is the explanation, and an arrow drawn
  // from a guessed position would be worse than none.
  it('draws nothing without a position', () => {
    const got = computeArrows(input({ position: null, targets: [cp('a', 56.5, 9)] }))

    expect(got).toEqual([])
  })

  // The overlay can mount before Leaflet has a container.
  it('draws nothing before the map exists', () => {
    const got = computeArrows(
      input({ viewportSize: () => null, targets: [cp('a', 56.5, 9)] }),
    )

    expect(got).toEqual([])
  })

  // Regression (bug reported 2026-09-15): arrows must survive the patrol's own position leaving the screen.
  //
  // The first implementation measured from the patrol and returned nothing when that origin was outside the
  // box — so a pan or a zoom-out made every arrow disappear at once, exactly when the user was looking around
  // for their next post. Placement is measured from the viewport centre now, which is always inside it.
  it('still draws arrows when the patrol’s own position is off screen', () => {
    const here = { lat: 56, lng: 9, accuracy: 10 }
    const got = computeArrows(
      input({
        position: here,
        // A projection that puts the patrol far off the left edge, as a pan would.
        project: (lat, lng) => ({
          x: -500 + (lng - here.lng) * 2000,
          y: H / 2 - (lat - here.lat) * 2000,
        }),
        targets: [cp('a', 56.5, 9)],
      }),
    )

    expect(got).toHaveLength(1)
    expect(got[0].id).toBe('a')
  })

  it('draws one arrow per off-screen post in the line', () => {
    const got = computeArrows(
      input({
        targets: [cp('a', 56.5, 9), cp('b', 55.5, 9), cp('c', 56, 9.5)],
      }),
    )

    expect(got.map((a) => a.id)).toEqual(['a', 'b', 'c'])
  })

  // Every arrow must be fully on screen: one drawn half outside is an arrow the patrol cannot read or tap.
  it('keeps every arrow inside the viewport', () => {
    const got = computeArrows(
      input({
        targets: [
          cp('n', 57, 9),
          cp('s', 55, 9),
          cp('e', 56, 10),
          cp('w', 56, 8),
          cp('ne', 57, 10),
        ],
      }),
    )

    expect(got).toHaveLength(5)
    for (const arrow of got) {
      expect(arrow.x).toBeGreaterThanOrEqual(ARROW_RADIUS)
      expect(arrow.x).toBeLessThanOrEqual(W - ARROW_RADIUS)
      expect(arrow.y).toBeGreaterThanOrEqual(ARROW_RADIUS)
      expect(arrow.y).toBeLessThanOrEqual(H - ARROW_RADIUS)
    }
  })

  // The bug reported in task 276, as a test. An arrow leaving the top edge where nothing floats must **hug the
  // edge** — the first version pushed it 112 px inward for controls that were not there. "Just inside the
  // viewport" is a near-side within a few px of the edge.
  it('hugs the edge where no control floats', () => {
    // Due north exits the top edge, at the horizontal centre — far from the top-right stack.
    const [arrow] = computeArrows(input({ keepOut: ZONES, targets: [cp('n', 57, 9)] }))

    expect(arrow).toBeDefined()
    // The arrow's centre is within a radius-plus-small-margin of the top edge, not a control-band inside it.
    expect(arrow.y).toBeLessThanOrEqual(ARROW_RADIUS + 12)
  })

  // Task 264's rule, preserved through the fix: an arrow that would sit under the top-right stack slides along
  // the edge to clear it, rather than being pushed into mid-air.
  it('slides an arrow along the edge to clear the top-right controls', () => {
    // A post up and well to the right exits near the top-right corner, under the control stack.
    const stack = ZONES[0]
    const [arrow] = computeArrows(input({ keepOut: ZONES, targets: [cp('ne', 57, 12)] }))

    expect(arrow).toBeDefined()
    // Cleared the stack: either left of it along the top edge, or below it down the right edge.
    const clearedHorizontally = arrow.x <= stack.x - ARROW_RADIUS + 0.01
    const clearedVertically = arrow.y >= stack.y + stack.height + ARROW_RADIUS - 0.01
    expect(clearedHorizontally || clearedVertically).toBe(true)
  })

  // And having cleared it, the arrow still hugs the edge — sliding is *along* the edge, not inward.
  it('still hugs the edge after sliding past a control', () => {
    const [arrow] = computeArrows(input({ keepOut: ZONES, targets: [cp('ne', 57, 12)] }))

    expect(arrow).toBeDefined()
    const nearTop = arrow.y <= ARROW_RADIUS + 12
    const nearRight = arrow.x >= W - ARROW_RADIUS - 12
    expect(nearTop || nearRight).toBe(true)
  })

  // No control on an edge means no adjustment at all: the arrow sits exactly where the geometry put it.
  it('does not move an arrow that is already clear of every control', () => {
    const withZones = computeArrows(input({ keepOut: ZONES, targets: [cp('w', 56, 8)] }))
    const without = computeArrows(input({ targets: [cp('w', 56, 8)] }))

    expect(withZones[0].x).toBeCloseTo(without[0].x, 6)
    expect(withZones[0].y).toBeCloseTo(without[0].y, 6)
  })

  // Sliding keeps the bearing and distance untouched — they are facts about the ground, not the screen.
  it('does not change bearing or distance when it slides an arrow', () => {
    const [slid] = computeArrows(input({ keepOut: ZONES, targets: [cp('ne', 57, 12)] }))
    const [free] = computeArrows(input({ targets: [cp('ne', 57, 12)] }))

    expect(slid.bearing).toBeCloseTo(free.bearing, 6)
    expect(slid.distance).toBe(free.distance)
  })

  // Every arrow stays on screen even with the zones applied, from all directions.
  it('keeps every arrow on screen with the control zones applied', () => {
    const got = computeArrows(
      input({
        keepOut: ZONES,
        targets: [cp('n', 57, 9), cp('s', 55, 9), cp('e', 56, 10), cp('w', 56, 8)],
      }),
    )

    expect(got).toHaveLength(4)
    for (const arrow of got) {
      expect(arrow.x).toBeGreaterThanOrEqual(ARROW_RADIUS)
      expect(arrow.x).toBeLessThanOrEqual(W - ARROW_RADIUS)
      expect(arrow.y).toBeGreaterThanOrEqual(ARROW_RADIUS)
      expect(arrow.y).toBeLessThanOrEqual(H - ARROW_RADIUS)
    }
  })

  // The non-visual route to the same information: the drawer carries the full list, and this is what a screen
  // reader gets from the map itself. Danish, with a compass point rather than a bearing in degrees.
  it('labels each arrow in Danish with distance and compass point', () => {
    const got = computeArrows(input({ targets: [cp('a', 56.5, 9)] }))

    expect(got[0].label).toContain('Post a')
    expect(got[0].label).toContain('mod nord')
    expect(got[0].label).toMatch(/\d/)
  })

  it('formats the distance the way a patrol reads it', () => {
    const here = { lat: 56, lng: 9, accuracy: 10 }

    // Zoomed right in, as a patrol walking has it: a post 440 m away is well off the screen.
    const near = computeArrows(
      input({
        project: projectionAround(here, { width: W, height: H }, 200_000),
        targets: [cp('a', 56.004, 9)],
      }),
    )
    const far = computeArrows(input({ targets: [cp('b', 56.5, 9)] }))

    expect(near).toHaveLength(1)
    expect(near[0].distance).toMatch(/^\d+0 m$/)
    expect(far[0].distance).toMatch(/^\d+,\d km$/)
  })

  // The bearing is the ground direction, not a screen angle: it must not change when the map is panned or the
  // window reshaped, only when the patrol or the target moves.
  it('reports a ground bearing that does not depend on the viewport', () => {
    const here = { lat: 56, lng: 9, accuracy: 10 }
    const wideSize = { width: 1200, height: 400 }

    const wide = computeArrows(
      input({
        targets: [cp('a', 56.5, 9)],
        viewportSize: () => wideSize,
        project: projectionAround(here, wideSize),
      }),
    )
    const tall = computeArrows(input({ targets: [cp('a', 56.5, 9)] }))

    expect(wide).toHaveLength(1)
    expect(tall).toHaveLength(1)
    expect(wide[0].bearing).toBeCloseTo(tall[0].bearing, 6)
  })
})

// --- Every post in the next line (task 275, reversing task 273) ---------------------------------------
//
// A postlinje holds several posts — an A and a B — and the patrol heads for the *line*, choosing which post
// when they arrive. Task 273 had the app nominate the nearest one; the product owner corrected that: show them
// all and leave the choice with the people walking.

describe('computeArrows, a whole line', () => {
  it('draws an arrow for every post in the line', () => {
    const got = computeArrows(
      input({
        targets: [cp('4a', 56.5, 9), cp('4b', 56.6, 9)],
      }),
    )

    expect(got.map((a) => a.id)).toEqual(['4a', '4b'])
  })

  // The regression this reverses: the app must not silently pick one post of the line for the patrol.
  it('does not nominate a single post for the patrol', () => {
    const near = cp('4b', 56.3, 9)
    const far = cp('4a', 56.9, 9)

    const got = computeArrows(input({ targets: [far, near] }))

    expect(got).toHaveLength(2)
  })

  // Each arrow carries its own distance, so the patrol can compare the alternatives — which is the point of
  // showing both.
  it('gives each post its own distance', () => {
    const got = computeArrows(
      input({ targets: [cp('4a', 56.9, 9), cp('4b', 56.3, 9)] }),
    )

    expect(got).toHaveLength(2)
    expect(got[0].distance).not.toBe(got[1].distance)
  })

  it('ignores a post it cannot project rather than drawing a blank arrow', () => {
    const got = computeArrows(
      input({
        project: (lat, lng) => (lat === 56.5 ? null : { x: -900, y: 400 }),
        targets: [cp('unprojectable', 56.5, 9), cp('a', 55.5, 9)],
      }),
    )

    expect(got.map((a) => a.id)).toEqual(['a'])
  })

  // A post already on screen needs no arrow — but a *different* post in the same line still does. Worth
  // asserting together, because skipping the wrong one is silent.
  it('skips only the post that is already on screen', () => {
    const got = computeArrows(
      input({
        targets: [cp('near', 56.01, 9.01), cp('far', 56.5, 9)],
      }),
    )

    expect(got.map((a) => a.id)).toEqual(['far'])
  })
})

// --- Stability during pan and zoom (bug reported 2026-09-15) -------------------------------------------
//
// Two symptoms were reported: arrows disappearing from time to time, and their placement jumping around.
// Both came from measuring placement from the patrol's own position, which moves across the screen during a
// drag and leaves it entirely on a zoom-out. Placement is measured from the viewport centre instead.

describe('computeArrows, panning and zooming', () => {
  const here = { lat: 56, lng: 9, accuracy: 10 }

  // A pan is a shift of the projection. Simulated by offsetting the projected points, which is exactly what
  // Leaflet does as the map slides.
  function pannedProjection(offsetX: number, offsetY: number) {
    return (lat: number, lng: number) => ({
      x: W / 2 + (lng - here.lng) * 2000 + offsetX,
      y: H / 2 - (lat - here.lat) * 2000 + offsetY,
    })
  }

  // The disappearing symptom, as a sweep rather than a single case: an arrow must be present at every step of a
  // long drag, not merely at the start and end.
  //
  // The invariant is "an arrow exists **whenever the post is off screen**" — not "an arrow always exists". My
  // first version of this test asserted the latter and failed, correctly: panning *towards* a post eventually
  // brings it into view, and an arrow pointing at a visible marker is the noise this code deliberately
  // suppresses. So the post's own projected position decides what to expect.
  it('keeps the arrow through a long pan, for as long as the post is off screen', () => {
    const post = cp('a', 56.4, 9.1)

    for (let offset = -1200; offset <= 1200; offset += 100) {
      for (const [dx, dy] of [
        [offset, 0],
        [0, offset],
        [offset, offset],
      ] as const) {
        const project = pannedProjection(dx, dy)
        const got = computeArrows(input({ project, targets: [post] }))

        const onScreen = isInside(project(post.lat, post.lng), W, H, ARROW_RADIUS)
        const where = `panned ${dx},${dy}`

        if (onScreen) {
          expect(got, `${where}: post is visible, so no arrow`).toHaveLength(0)
        } else {
          expect(got, `${where}: post is off screen, so an arrow is required`).toHaveLength(1)
        }
      }
    }
  })

  // The jumping symptom. A small pan must move the arrow by a comparable amount — with the old geometry the
  // ray pivoted around the patrol's dot and the crossing point raced along the edge.
  it('moves the arrow smoothly, not faster than the map', () => {
    const step = 10
    let previous: { x: number; y: number } | null = null

    for (let offset = 0; offset <= 300; offset += step) {
      const [arrow] = computeArrows(
        input({ project: pannedProjection(offset, 0), targets: [cp('a', 56.4, 9.1)] }),
      )
      expect(arrow).toBeDefined()

      if (previous) {
        const moved = Math.hypot(arrow.x - previous.x, arrow.y - previous.y)
        // Generous, because an arrow legitimately turns a corner: what it rules out is the old behaviour,
        // where a 10px drag could throw the arrow the length of an edge.
        expect(moved, `a ${step}px pan moved the arrow ${moved.toFixed(1)}px`).toBeLessThan(step * 6)
      }
      previous = { x: arrow.x, y: arrow.y }
    }
  })

  // A zoom changes the scale rather than the offset. The arrow must stay put through it — the post has not
  // moved, and neither has the patrol.
  it('keeps the arrow through a zoom-out that takes the patrol off screen', () => {
    for (const pxPerDegree of [200_000, 50_000, 8_000, 2_000, 500]) {
      const got = computeArrows(
        input({
          project: (lat, lng) => ({
            // Deliberately off-centre, so zooming out sweeps the patrol out of the viewport.
            x: W / 2 + (lng - here.lng) * pxPerDegree - 600,
            y: H / 2 - (lat - here.lat) * pxPerDegree,
          }),
          targets: [cp('a', 56.5, 9)],
        }),
      )

      expect(got, `at ${pxPerDegree}px per degree`).toHaveLength(1)
    }
  })

  // "Just inside the viewport" — the reported expectation, asserted across a full circle of directions so no
  // single quadrant can pass by luck.
  it('places every arrow just inside the viewport, from any direction', () => {
    for (let degrees = 0; degrees < 360; degrees += 15) {
      const radians = (degrees * Math.PI) / 180
      // A post well outside the viewport in that direction.
      const post = cp('a', here.lat + Math.cos(radians) * 0.5, here.lng + Math.sin(radians) * 0.5)

      const [arrow] = computeArrows(input({ targets: [post] }))
      expect(arrow, `${degrees}°`).toBeDefined()

      // Inside the box...
      expect(arrow.x).toBeGreaterThanOrEqual(ARROW_RADIUS)
      expect(arrow.x).toBeLessThanOrEqual(W - ARROW_RADIUS)
      expect(arrow.y).toBeGreaterThanOrEqual(ARROW_RADIUS)
      expect(arrow.y).toBeLessThanOrEqual(H - ARROW_RADIUS)

      // ...and against an edge rather than adrift in the middle of it.
      const nearAnEdge =
        arrow.x <= ARROW_RADIUS * 2 ||
        arrow.x >= W - ARROW_RADIUS * 2 ||
        arrow.y <= ARROW_RADIUS * 2 ||
        arrow.y >= H - ARROW_RADIUS * 2
      expect(nearAnEdge, `${degrees}° placed the arrow at ${arrow.x},${arrow.y}`).toBe(true)
    }
  })

  // The bearing and the distance are facts about the ground: dragging the map must not change either.
  it('does not change the bearing or distance when the map is panned', () => {
    const still = computeArrows(input({ targets: [cp('a', 56.5, 9.2)] }))
    const panned = computeArrows(
      input({ project: pannedProjection(400, -250), targets: [cp('a', 56.5, 9.2)] }),
    )

    expect(panned[0].bearing).toBeCloseTo(still[0].bearing, 6)
    expect(panned[0].distance).toBe(still[0].distance)
    expect(panned[0].label).toBe(still[0].label)
  })
})
