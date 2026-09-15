import { describe, expect, it } from 'vitest'

import {
  ARROW_RADIUS,
  computeArrows,
  type ArrowInput,
} from '@/components/map/arrowPlacement'
import { isInside } from '@/components/map/arrowGeometry'
import { arrowKeepOut } from '@/config/map'

// A notched phone's reading, which is the case a desktop-tuned constant would get wrong.
const KEEP_OUT = arrowKeepOut({ top: 59, bottom: 34 })
import type { Checkpoint } from '@/stores/checkpoints.store'

const W = 400
const H = 800

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

  // Task 264's rule, tested here because this is where it is implemented: an arrow under the locate button is
  // worse than no arrow, since it is invisible *and* steals the tap.
  it('keeps arrows out of the bands the floating controls occupy', () => {
    const got = computeArrows(
      input({
        insets: KEEP_OUT,
        targets: [cp('n', 57, 9), cp('s', 55, 9), cp('e', 56, 10), cp('w', 56, 8)],
      }),
    )

    expect(got).toHaveLength(4)
    for (const arrow of got) {
      expect(arrow.y).toBeGreaterThanOrEqual(KEEP_OUT.top + ARROW_RADIUS)
      expect(arrow.y).toBeLessThanOrEqual(H - KEEP_OUT.bottom - ARROW_RADIUS)
    }
  })

  // Clamped rather than dropped: direction is approximate at the edge anyway, so a few pixels of slide costs
  // almost nothing, while losing the arrow loses the only thing telling the patrol which way to walk.
  it('clamps an arrow into the band rather than discarding it', () => {
    const withoutInsets = computeArrows(input({ targets: [cp('n', 57, 9)] }))
    const withInsets = computeArrows(
      input({ insets: KEEP_OUT, targets: [cp('n', 57, 9)] }),
    )

    expect(withoutInsets).toHaveLength(1)
    expect(withInsets).toHaveLength(1)
    // The arrow moved down out of the top band, and kept its direction.
    expect(withInsets[0].y).toBeGreaterThan(withoutInsets[0].y)
    expect(withInsets[0].bearing).toBeCloseTo(withoutInsets[0].bearing, 6)
  })

  // A viewport shorter than the keep-out bands is possible in landscape. Centring what is left beats clamping
  // against contradictory bounds, which would pin the arrow off screen entirely.
  it('survives a viewport shorter than its keep-out bands', () => {
    const here = { lat: 56, lng: 9, accuracy: 10 }
    const shortSize = { width: W, height: 200 }

    const got = computeArrows(
      input({
        insets: KEEP_OUT,
        viewportSize: () => shortSize,
        project: projectionAround(here, shortSize),
        targets: [cp('n', 57, 9)],
      }),
    )

    for (const arrow of got) {
      expect(arrow.y).toBeGreaterThanOrEqual(0)
      expect(arrow.y).toBeLessThanOrEqual(200)
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
