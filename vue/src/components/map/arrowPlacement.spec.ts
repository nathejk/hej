import { describe, expect, it } from 'vitest'

import { ARROW_RADIUS, computeArrows, type ArrowInput } from '@/components/map/arrowPlacement'
import { ARROW_KEEP_OUT } from '@/config/map'
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

  // The patrol has panned away from themselves: there is no honest edge crossing to compute from an origin
  // that is not in the box.
  it('draws nothing when the patrol’s own position is off screen', () => {
    const here = { lat: 56, lng: 9, accuracy: 10 }
    const got = computeArrows(
      input({
        position: here,
        // A projection that puts the patrol far off the left edge.
        project: (lat, lng) => ({
          x: -500 + (lng - here.lng) * 2000,
          y: H / 2 - (lat - here.lat) * 2000,
        }),
        targets: [cp('a', 56.5, 9)],
      }),
    )

    expect(got).toEqual([])
  })

  it('draws one arrow per off-screen target', () => {
    const got = computeArrows(
      input({ targets: [cp('a', 56.5, 9), cp('b', 55.5, 9), cp('c', 56, 9.5)] }),
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
        insets: ARROW_KEEP_OUT,
        targets: [cp('n', 57, 9), cp('s', 55, 9), cp('e', 56, 10), cp('w', 56, 8)],
      }),
    )

    expect(got).toHaveLength(4)
    for (const arrow of got) {
      expect(arrow.y).toBeGreaterThanOrEqual(ARROW_KEEP_OUT.top + ARROW_RADIUS)
      expect(arrow.y).toBeLessThanOrEqual(H - ARROW_KEEP_OUT.bottom - ARROW_RADIUS)
    }
  })

  // Clamped rather than dropped: direction is approximate at the edge anyway, so a few pixels of slide costs
  // almost nothing, while losing the arrow loses the only thing telling the patrol which way to walk.
  it('clamps an arrow into the band rather than discarding it', () => {
    const withoutInsets = computeArrows(input({ targets: [cp('n', 57, 9)] }))
    const withInsets = computeArrows(
      input({ insets: ARROW_KEEP_OUT, targets: [cp('n', 57, 9)] }),
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
        insets: ARROW_KEEP_OUT,
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
