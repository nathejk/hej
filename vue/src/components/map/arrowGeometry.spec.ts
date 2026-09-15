import { describe, expect, it } from 'vitest'

import {
  bearingDegrees,
  compassDanish,
  distanceMetres,
  edgeIntersection,
  formatDistance,
  isInside,
} from '@/components/map/arrowGeometry'

describe('distanceMetres', () => {
  it('is zero for the same point', () => {
    expect(distanceMetres({ lat: 56.1, lng: 9.5 }, { lat: 56.1, lng: 9.5 })).toBe(0)
  })

  // A degree of latitude is ~111.2 km anywhere. A wrong earth radius, or degrees left unconverted, would show
  // up here as an answer off by orders of magnitude — the failure that would put "3400 km" on an arrow.
  it('matches the known length of a degree of latitude', () => {
    const d = distanceMetres({ lat: 56, lng: 9 }, { lat: 57, lng: 9 })

    expect(d).toBeGreaterThan(111_000)
    expect(d).toBeLessThan(111_400)
  })

  // A degree of longitude shrinks with the cosine of the latitude: ~62 km at 56°N, not 111 km. Getting this
  // wrong is the classic flat-earth bug, and at this event's latitude it would overstate every east-west
  // distance by 80 %.
  it('accounts for longitude converging towards the pole', () => {
    const d = distanceMetres({ lat: 56, lng: 9 }, { lat: 56, lng: 10 })

    expect(d).toBeGreaterThan(61_000)
    expect(d).toBeLessThan(63_000)
  })

  it('is symmetric', () => {
    const a = { lat: 56.1382, lng: 9.5521 }
    const b = { lat: 56.2311, lng: 9.5216 }

    expect(distanceMetres(a, b)).toBeCloseTo(distanceMetres(b, a), 6)
  })

  // Two posts a few kilometres apart, which is the scale this is actually used at.
  it('is plausible over event distances', () => {
    const d = distanceMetres({ lat: 56.1382, lng: 9.5521 }, { lat: 56.1804, lng: 9.4812 })

    expect(d).toBeGreaterThan(5_000)
    expect(d).toBeLessThan(7_000)
  })
})

describe('bearingDegrees', () => {
  // The four cardinals, because a bearing off by 90° is the failure that walks a patrol the wrong way while
  // looking entirely reasonable on screen.
  //
  // North and south are exact: a meridian *is* a great circle. East and west are not, and that is geodesy
  // rather than a bug — a parallel is not a great circle, so the shortest path due "east" sets off slightly
  // north of 90° and curves. At 56°N over one degree of longitude that is about 0.4°, which is far below
  // what an arrow on a phone can express. My first version of this test asserted 90° to two decimals and
  // failed; the tolerance is the thing that was wrong.
  it('points north, east, south and west correctly', () => {
    const here = { lat: 56, lng: 9 }

    expect(bearingDegrees(here, { lat: 57, lng: 9 })).toBeCloseTo(0, 6)
    expect(bearingDegrees(here, { lat: 55, lng: 9 })).toBeCloseTo(180, 6)

    expect(bearingDegrees(here, { lat: 56, lng: 10 })).toBeGreaterThan(89)
    expect(bearingDegrees(here, { lat: 56, lng: 10 })).toBeLessThanOrEqual(90)
    expect(bearingDegrees(here, { lat: 56, lng: 8 })).toBeGreaterThanOrEqual(270)
    expect(bearingDegrees(here, { lat: 56, lng: 8 })).toBeLessThan(271)
  })

  // Normalised into [0, 360). A negative angle leaking out is how a north-west arrow becomes a south-east
  // one.
  it('never returns a negative angle', () => {
    const here = { lat: 56, lng: 9 }

    for (const to of [
      { lat: 56.5, lng: 8.5 },
      { lat: 55.5, lng: 8.5 },
      { lat: 56, lng: 8.999 },
    ]) {
      const b = bearingDegrees(here, to)
      expect(b).toBeGreaterThanOrEqual(0)
      expect(b).toBeLessThan(360)
    }
  })

  it('is roughly north-east for a point up and to the right', () => {
    const b = bearingDegrees({ lat: 56, lng: 9 }, { lat: 56.5, lng: 9.5 })

    expect(b).toBeGreaterThan(20)
    expect(b).toBeLessThan(70)
  })
})

describe('compassDanish', () => {
  it('names the eight points', () => {
    expect(compassDanish(0)).toBe('nord')
    expect(compassDanish(45)).toBe('nordøst')
    expect(compassDanish(90)).toBe('øst')
    expect(compassDanish(135)).toBe('sydøst')
    expect(compassDanish(180)).toBe('syd')
    expect(compassDanish(225)).toBe('sydvest')
    expect(compassDanish(270)).toBe('vest')
    expect(compassDanish(315)).toBe('nordvest')
  })

  // 350° is north, not "nordvest" and certainly not an index off the end of the array.
  it('wraps back to north near 360', () => {
    expect(compassDanish(350)).toBe('nord')
    expect(compassDanish(360)).toBe('nord')
    expect(compassDanish(-10)).toBe('nord')
  })
})

describe('formatDistance', () => {
  // A walking patrol cannot act on single metres, and a number that changes every step reads as noise.
  it('rounds metres to the nearest ten below a kilometre', () => {
    expect(formatDistance(0)).toBe('0 m')
    expect(formatDistance(143)).toBe('140 m')
    expect(formatDistance(999)).toBe('1000 m')
  })

  it('uses a Danish decimal comma above a kilometre', () => {
    expect(formatDistance(3400)).toBe('3,4 km')
    expect(formatDistance(12_050)).toBe('12,1 km')
  })

  it('switches unit at exactly a kilometre', () => {
    expect(formatDistance(1000)).toBe('1,0 km')
  })
})

describe('edgeIntersection', () => {
  const W = 400
  const H = 800
  const INSET = 20
  const centre = { x: 200, y: 400 }

  // Straight up from the centre must land on the top edge, at the inset, directly above.
  it('places a due-north target on the top edge', () => {
    const p = edgeIntersection(centre, { x: 200, y: -500 }, W, H, INSET)

    expect(p).not.toBeNull()
    expect(p!.y).toBeCloseTo(INSET, 5)
    expect(p!.x).toBeCloseTo(200, 5)
  })

  it('places a due-east target on the right edge', () => {
    const p = edgeIntersection(centre, { x: 900, y: 400 }, W, H, INSET)

    expect(p!.x).toBeCloseTo(W - INSET, 5)
    expect(p!.y).toBeCloseTo(400, 5)
  })

  // The point of intersecting the real ray rather than pinning to an edge midpoint: the arrow slides along
  // the edge as the direction changes, which is what makes it read as a direction.
  it('slides along the edge as the direction changes', () => {
    const straightUp = edgeIntersection(centre, { x: 200, y: -500 }, W, H, INSET)!
    const upAndRight = edgeIntersection(centre, { x: 400, y: -500 }, W, H, INSET)!

    expect(upAndRight.x).toBeGreaterThan(straightUp.x)
  })

  // A 45° target must leave through whichever edge is nearer in ray terms — for a tall viewport, the side.
  it('leaves through the nearer edge in ray terms', () => {
    const p = edgeIntersection(centre, { x: 1000, y: -600 }, W, H, INSET)!

    // The box is 400 wide and 800 tall, so from the centre the horizontal wall is closer.
    expect(p.x).toBeCloseTo(W - INSET, 5)
  })

  it('keeps the arrow inside the inset on every side', () => {
    for (const target of [
      { x: 200, y: -900 },
      { x: 200, y: 1900 },
      { x: -900, y: 400 },
      { x: 1900, y: 400 },
      { x: -900, y: -900 },
    ]) {
      const p = edgeIntersection(centre, target, W, H, INSET)!
      expect(isInside(p, W, H, INSET - 0.001)).toBe(true)
    }
  })

  // No origin inside the box means no sensible crossing — and nothing useful to draw, rather than an arrow
  // pointing at a guess.
  it('returns null when the origin is outside the box', () => {
    expect(edgeIntersection({ x: -10, y: 400 }, { x: 200, y: 400 }, W, H, INSET)).toBeNull()
  })

  it('returns null when target and origin coincide', () => {
    expect(edgeIntersection(centre, centre, W, H, INSET)).toBeNull()
  })

  // A viewport too small for the inset would otherwise produce a negative-sized box and a nonsense point.
  // Worth guarding: a phone in landscape with the keyboard up is genuinely short.
  it('returns null when the viewport is smaller than the inset allows', () => {
    expect(edgeIntersection({ x: 10, y: 10 }, { x: 100, y: 10 }, 30, 30, 20)).toBeNull()
  })
})

describe('isInside', () => {
  it('respects the margin', () => {
    expect(isInside({ x: 5, y: 5 }, 100, 100, 10)).toBe(false)
    expect(isInside({ x: 50, y: 50 }, 100, 100, 10)).toBe(true)
    expect(isInside({ x: 90, y: 90 }, 100, 100, 10)).toBe(true)
    expect(isInside({ x: 91, y: 50 }, 100, 100, 10)).toBe(false)
  })
})
