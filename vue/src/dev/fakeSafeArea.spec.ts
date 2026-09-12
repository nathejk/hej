import { describe, expect, it } from 'vitest'

import { SIMULATED_INSETS } from '@/dev/fakeSafeArea'
import { insetVars } from '@/helpers/safeArea'

// These presets are only useful if they go through the *real* rule, so this checks them against
// `insetVars()` rather than against themselves. No DOM needed — which is exactly why that
// function was split out of `apply()` in the first place.

describe('simulated safe-area presets', () => {
  // The point of carrying a shortfall with the insets. An iPhone 16 in standalone reads
  // 59/0/34/0 with a 59px shortfall, and `insetVars` reduces the bottom inset by it — so the
  // real device gets `--sab: 0px`. A preset that invented a shortfall of 0 would show 34px of
  // bottom padding no iPhone actually gets, and "the bottom nav takes up too much space" would
  // look like a regression that had come back.
  it('reproduces the real iPhone result, including the bottom reduction', () => {
    const { insets, shortfall } = SIMULATED_INSETS['iphone-portrait']
    const vars = insetVars(insets, shortfall)

    expect(vars).toEqual({
      '--sat': '59px',
      '--sar': '0px',
      '--sab': '0px',
      '--sal': '0px',
    })
  })

  // The insets nobody remembers, which is the argument for the preset existing.
  it('produces left and right insets in landscape', () => {
    const { insets, shortfall } = SIMULATED_INSETS['iphone-landscape']
    const vars = insetVars(insets, shortfall)

    expect(vars).toMatchObject({ '--sal': '47px', '--sar': '47px', '--sat': '0px' })
  })

  it('produces a top inset on iPad', () => {
    const { insets, shortfall } = SIMULATED_INSETS.ipad
    expect(insetVars(insets, shortfall)).toMatchObject({ '--sat': '24px' })
  })

  // Not a property of the presets but of the rule they rely on, and the reason `setSafeAreaPreset`
  // has to remove the inline properties when switching back to `none`: on a laptop the real
  // reading is all-zero, `insetVars` discards it, and the simulated values would otherwise
  // survive being switched off.
  it('discards an all-zero reading, which is why switching off must un-write the properties', () => {
    expect(insetVars({ top: 0, right: 0, bottom: 0, left: 0 }, 0)).toBeNull()
  })
})
