import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { arrowKeepOutZones } from '@/config/map'
import { ARROW_RADIUS } from '@/components/map/arrowPlacement'

// Keeping edge arrows clear of the map's floating controls (PRD 016, task 264; corrected task 276).
//
// The controls sit in corners — the layer/locate stack top-right, the registrations handle bottom-centre —
// not across whole edges. The first version reserved full-width bands and pushed left-edge arrows into
// mid-air; these zones are corner-shaped so the rest of every edge stays free.

const SIZE = { width: 400, height: 800 }

function zoneAt(zones: ReturnType<typeof arrowKeepOutZones>, corner: 'top-right' | 'bottom-centre') {
  // The two zones, identified by where they sit rather than by index.
  return zones.find((z) => {
    if (corner === 'top-right') return z.x + z.width >= SIZE.width - 1 && z.y <= 1
    return z.x < SIZE.width / 2 && z.x + z.width > SIZE.width / 2 && z.y + z.height >= SIZE.height - 1
  })
}

describe('arrowKeepOutZones', () => {
  it('reserves the top-right corner for the control stack', () => {
    const stack = zoneAt(arrowKeepOutZones(SIZE, { top: 0, bottom: 0 }), 'top-right')

    expect(stack).toBeDefined()
    // Anchored to the right edge and the top.
    expect(stack!.x + stack!.width).toBeCloseTo(SIZE.width, 0)
    expect(stack!.y).toBe(0)
  })

  it('reserves the bottom centre for the registrations handle', () => {
    const handle = zoneAt(arrowKeepOutZones(SIZE, { top: 0, bottom: 0 }), 'bottom-centre')

    expect(handle).toBeDefined()
    // Straddles the horizontal centre and reaches the bottom edge.
    expect(handle!.x).toBeLessThan(SIZE.width / 2)
    expect(handle!.x + handle!.width).toBeGreaterThan(SIZE.width / 2)
    expect(handle!.y + handle!.height).toBeCloseTo(SIZE.height, 0)
  })

  // The left of the top edge is deliberately NOT reserved. Reserving it for the sometimes-present notices is
  // exactly what made left-edge arrows float (task 276), and a rare overlap with a transient notice is the
  // better trade.
  it('leaves the top-left corner free', () => {
    const zones = arrowKeepOutZones(SIZE, { top: 0, bottom: 0 })

    const coversTopLeft = zones.some((z) => z.x <= 0 && z.y <= 0 && z.width > 0 && z.height > 0)
    expect(coversTopLeft).toBe(false)
  })

  // The notch. `--sat` is 59 px on the maintainer's iPhone and 0 on desktop, so the top-right zone grows
  // downward by the inset rather than being a fixed height tuned on one device.
  it('extends the top zone by the safe-area inset', () => {
    const flat = zoneAt(arrowKeepOutZones(SIZE, { top: 0, bottom: 0 }), 'top-right')!
    const notched = zoneAt(arrowKeepOutZones(SIZE, { top: 59, bottom: 34 }), 'top-right')!

    expect(notched.height).toBe(flat.height + 59)
    expect(notched.y).toBe(0)
  })

  it('lifts the bottom zone by the safe-area inset', () => {
    const flat = zoneAt(arrowKeepOutZones(SIZE, { top: 0, bottom: 0 }), 'bottom-centre')!
    const notched = zoneAt(arrowKeepOutZones(SIZE, { top: 59, bottom: 34 }), 'bottom-centre')!

    // The zone reaches the bottom in both, so a larger inset makes it taller and starts it higher.
    expect(notched.height).toBe(flat.height + 34)
    expect(notched.y).toBeLessThan(flat.y)
  })

  // The safe-area properties can read as anything before the first paint (see `safeArea.ts`). A negative or
  // non-finite value must not throw the zone off screen.
  it('floors a nonsense safe-area reading at zero', () => {
    const zones = arrowKeepOutZones(SIZE, { top: -100, bottom: Number.NaN })

    for (const z of zones) {
      expect(Number.isFinite(z.x)).toBe(true)
      expect(Number.isFinite(z.y)).toBe(true)
      expect(Number.isFinite(z.width)).toBe(true)
      expect(Number.isFinite(z.height)).toBe(true)
    }
  })

  // The reserved corners must actually clear a 44 px touch target, or avoiding them is decorative.
  it('reserves enough to clear a touch target', () => {
    const stack = zoneAt(arrowKeepOutZones(SIZE, { top: 0, bottom: 0 }), 'top-right')!
    const handle = zoneAt(arrowKeepOutZones(SIZE, { top: 0, bottom: 0 }), 'bottom-centre')!

    expect(stack.width).toBeGreaterThanOrEqual(44)
    expect(stack.height).toBeGreaterThanOrEqual(44)
    expect(handle.height).toBeGreaterThanOrEqual(44)
    // The arrow is placed by its centre, so a zone has to be at least a radius deep to be worth avoiding.
    expect(stack.height).toBeGreaterThan(ARROW_RADIUS)
  })
})

// Structural assertions on the overlay, following `layout.spec.ts` and `offlineIndicator.spec.ts`: assert the
// cause rather than the symptom, because the symptom only shows on a phone in a forest.
describe('the arrow overlay does not steal taps', () => {
  const source = readFileSync(
    fileURLToPath(new URL('./EdgeArrows.vue', import.meta.url)),
    'utf8',
  )

  it('is transparent to pointer events except on the arrows themselves', () => {
    expect(source).toContain('pointer-events-none')
    expect(source).toContain('pointer-events-auto')
    expect(source.indexOf('pointer-events-none')).toBeLessThan(source.indexOf('pointer-events-auto'))
  })

  it('renders each arrow as a button with an accessible label', () => {
    expect(source).toContain('type="button"')
    expect(source).toContain(':aria-label="arrow.label"')
  })

  it('hides the decorative chevron from assistive technology', () => {
    expect(source).toContain('aria-hidden="true"')
  })
})
