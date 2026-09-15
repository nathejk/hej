import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { arrowKeepOut, CONTROL_EXTENT } from '@/config/map'
import { ARROW_RADIUS } from '@/components/map/arrowPlacement'

// Keeping edge arrows clear of the map's floating controls (PRD 016, task 264).
//
// `MapsView` floats the layer/locate stack top-right, the notices top-left and the registrations handle
// bottom-centre — all in the same `z-10` overlay layer the arrows live in. An arrow that lands under the locate
// button is worse than no arrow: it is invisible *and* it steals the tap.

describe('arrowKeepOut', () => {
  // The reason this is composed rather than hardcoded. `--sat` is 59 px on the maintainer's notched iPhone and
  // 0 on a desktop browser, so one fixed number either wastes half the screen or hides arrows behind the
  // controls.
  it('adds the device’s safe-area insets to the controls’ own extent', () => {
    const flat = arrowKeepOut({ top: 0, bottom: 0 })
    const notched = arrowKeepOut({ top: 59, bottom: 34 })

    expect(flat.top).toBe(CONTROL_EXTENT.top)
    expect(flat.bottom).toBe(CONTROL_EXTENT.bottom)
    expect(notched.top).toBe(CONTROL_EXTENT.top + 59)
    expect(notched.bottom).toBe(CONTROL_EXTENT.bottom + 34)
  })

  it('leaves the sides alone, because nothing floats there', () => {
    const insets = arrowKeepOut({ top: 59, bottom: 34 })

    expect(insets.left).toBe(CONTROL_EXTENT.sides)
    expect(insets.right).toBe(CONTROL_EXTENT.sides)
  })

  // The safe-area properties can read as anything before the first paint — `safeArea.ts` documents an all-zero
  // read on iOS standalone that had to be discarded. A negative value reaching here would pull arrows off
  // screen, so it is floored rather than trusted.
  it('floors a nonsense reading at zero', () => {
    const insets = arrowKeepOut({ top: -100, bottom: Number.NaN })

    expect(insets.top).toBe(CONTROL_EXTENT.top)
    expect(insets.bottom).toBeGreaterThanOrEqual(CONTROL_EXTENT.bottom)
    expect(Number.isNaN(insets.bottom)).toBe(false)
  })

  // The extents have to actually clear a 44 px touch target plus the spacing above it, or the whole exercise is
  // decorative. Two stacked controls at the top, one handle plus the nav at the bottom.
  it('is large enough to clear the controls it exists for', () => {
    const flat = arrowKeepOut({ top: 0, bottom: 0 })

    expect(flat.top).toBeGreaterThanOrEqual(44 * 2)
    expect(flat.bottom).toBeGreaterThanOrEqual(44 * 2)
  })

  // An arrow is placed by its centre, so the band has to leave room for its radius on both sides or the clamp
  // has nothing to clamp into. On the shortest viewport we support (an iPhone SE in landscape, ~320 px) that is
  // tight, which is exactly why it is asserted.
  it('leaves a usable band on a short viewport', () => {
    const insets = arrowKeepOut({ top: 0, bottom: 0 })
    const shortest = 320

    const band = shortest - insets.top - insets.bottom
    expect(band).toBeGreaterThan(ARROW_RADIUS * 2)
  })
})

// A structural assertion, following the precedent of `layout.spec.ts` and `offlineIndicator.spec.ts`: assert
// the cause rather than the symptom, because the symptom only appears on a phone in a forest.
describe('the arrow overlay does not steal taps', () => {
  const source = readFileSync(
    fileURLToPath(new URL('./EdgeArrows.vue', import.meta.url)),
    'utf8',
  )

  // The layer covers the whole map. If it swallowed pointer events the map would stop being draggable
  // everywhere except on the arrows — which is the opposite of the intent, and would be reported as "the map
  // froze".
  it('is transparent to pointer events except on the arrows themselves', () => {
    expect(source).toContain('pointer-events-none')
    expect(source).toContain('pointer-events-auto')

    // The order matters: none on the container, auto on the button.
    expect(source.indexOf('pointer-events-none')).toBeLessThan(
      source.indexOf('pointer-events-auto'),
    )
  })

  // Each arrow is a real button, so it is focusable and announced. A tappable `div` would look identical and be
  // unreachable without a pointer.
  it('renders each arrow as a button with an accessible label', () => {
    expect(source).toContain('type="button"')
    expect(source).toContain(':aria-label="arrow.label"')
  })

  // The chevron conveys direction visually; the label already says "mod nordøst". Announcing both would read
  // the same fact twice.
  it('hides the decorative chevron from assistive technology', () => {
    expect(source).toContain('aria-hidden="true"')
  })
})
