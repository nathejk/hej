import { describe, it, expect } from 'vitest'
import { handoutStatus, handoutSticker } from './handoutPresentation'

// handoutPresentation is pure, so it is tested here (vitest runs in node with no DOM). The row markup
// renders whatever these return; the guarantee that matters — "afleveret" names nobody — lives here.

describe('handoutStatus', () => {
  it('has no status line while the sheet is still held', () => {
    expect(handoutStatus({ stillHeld: true })).toBeNull()
  })

  it('reads "afleveret" once the sheet is no longer held', () => {
    expect(handoutStatus({ stillHeld: false })).toBe('afleveret')
  })

  // The whole point of this feature's privacy rule: a participant is never told which team a sheet moved
  // to. The label is a bare constant, so there is nothing to interpolate a team name into.
  it('never names another team', () => {
    const label = handoutStatus({ stillHeld: false }) ?? ''
    for (const banned of ['flyttet', 'til', 'hold', 'team', 'patrulje']) {
      expect(label.toLowerCase()).not.toContain(banned)
    }
  })
})

describe('handoutSticker', () => {
  it('returns the sticker number when there is one', () => {
    expect(handoutSticker({ qrId: '1042' })).toBe('1042')
  })

  // A synthesised handout (a skitse) has no code; the row must not leave a gap where the number would go,
  // so the caller renders the sticker line only when this is non-null.
  it('returns null for a QR-less (synthesised) handout', () => {
    expect(handoutSticker({ qrId: '' })).toBeNull()
  })
})
