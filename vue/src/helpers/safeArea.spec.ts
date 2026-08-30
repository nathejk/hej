import { describe, expect, it } from 'vitest'

import { insetVars, type Insets } from '@/helpers/safeArea'

const none: Insets = { top: 0, right: 0, bottom: 0, left: 0 }

describe('insetVars', () => {
  // THE REGRESSION (task 145). On iOS standalone the insets read as 0 until the first frame is
  // painted, and this runs before mount by design. Writing that reading produced
  // `--sat: 0px` — a static value shadowing a live `env()` seed that would have resolved to
  // 59px moments later — and nothing ever corrected it, because a cold launch is followed by no
  // resize or orientation change. The top bar therefore sat behind the status bar.
  it('writes nothing for an all-zero reading', () => {
    expect(insetVars(none, 0)).toBe(null)
    // Including when there is a shortfall: a device mid-launch reads zeros and has one.
    expect(insetVars(none, 59)).toBe(null)
  })

  it('passes the top inset through unchanged', () => {
    const vars = insetVars({ top: 59, right: 0, bottom: 34, left: 0 }, 0)
    expect(vars?.['--sat']).toBe('59px')
  })

  // The documented bottom rule: the shell already ends `shortfall` pixels above the screen
  // edge, so that much of the home-indicator inset is provably redundant padding.
  it('reduces the bottom inset by the viewport shortfall', () => {
    const vars = insetVars({ top: 59, right: 0, bottom: 34, left: 0 }, 59)
    expect(vars?.['--sab']).toBe('0px')
  })

  it('keeps the full bottom inset when the viewport fills the screen', () => {
    const vars = insetVars({ top: 59, right: 0, bottom: 34, left: 0 }, 0)
    expect(vars?.['--sab']).toBe('34px')
  })

  it('never produces negative padding', () => {
    const vars = insetVars({ top: 59, right: 0, bottom: 34, left: 0 }, 200)
    expect(vars?.['--sab']).toBe('0px')
  })

  // A landscape notch: the reading is not all-zero, so it is applied even though the top is 0.
  it('applies a reading whose top happens to be zero, when another edge is set', () => {
    const vars = insetVars({ top: 0, right: 59, bottom: 21, left: 59 }, 0)
    expect(vars).not.toBe(null)
    expect(vars?.['--sat']).toBe('0px')
    expect(vars?.['--sar']).toBe('59px')
    expect(vars?.['--sal']).toBe('59px')
  })
})
