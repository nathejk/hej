import { describe, it, expect } from 'vitest'
import { deltaText, minutesText, verdictBadge } from './scanVerdict'

// scanVerdict is pure, so it is tested here rather than through the component (vitest runs in node with no
// DOM). ScanList.vue renders whatever this returns; the decisions live here.

describe('deltaText', () => {
  it('rounds seconds to whole minutes', () => {
    expect(deltaText(12 * 60)).toBe('12 min for sent')
    // 90 s rounds up to 2, not truncates to 1.
    expect(deltaText(90)).toBe('2 min for sent')
  })

  it('distinguishes early from late by the sign', () => {
    expect(deltaText(5 * 60)).toBe('5 min for sent')
    expect(deltaText(-5 * 60)).toBe('5 min for tidligt')
  })

  it('drops the number inside the same minute as the boundary', () => {
    // A misleading "0 min for sent" would look like data; the direction alone is honest.
    expect(deltaText(20)).toBe('for sent')
    expect(deltaText(-20)).toBe('for tidligt')
  })
})

describe('minutesText', () => {
  it('renders whole minutes with the abbreviated unit', () => {
    expect(minutesText(45 * 60)).toBe('45 min.')
    expect(minutesText(0)).toBe('0 min.')
  })

  it('rounds to the nearest minute', () => {
    expect(minutesText(90)).toBe('2 min.')
  })
})

describe('verdictBadge', () => {
  it('shows a green "På tid" badge on time', () => {
    expect(verdictBadge({ kind: 'checkpoint', onTime: true, deltaSeconds: 0, spentSeconds: null })).toEqual({
      text: 'På tid',
      tone: 'success',
    })
  })

  // A relative leg's elapsed time is the actual answer to "how did we do": a bare "På tid" tells a patrol
  // only that they beat a deadline they cannot see.
  it('appends the elapsed minutes on a relative leg', () => {
    expect(
      verdictBadge({ kind: 'checkpoint', onTime: true, deltaSeconds: 0, spentSeconds: 45 * 60 }),
    ).toEqual({ text: 'På tid: 45 min.', tone: 'success' })
  })

  it('rounds the elapsed minutes', () => {
    const badge = verdictBadge({
      kind: 'checkpoint',
      onTime: true,
      deltaSeconds: 0,
      spentSeconds: 45 * 60 + 40,
    })
    expect(badge?.text).toBe('På tid: 46 min.')
  })

  // Zero is a real answer (scanned at the anchoring post's own line), and must still read as a duration
  // rather than falling back to the bare form.
  it('renders a zero-minute leg as a duration', () => {
    const badge = verdictBadge({ kind: 'checkpoint', onTime: true, deltaSeconds: 0, spentSeconds: 0 })
    expect(badge?.text).toBe('På tid: 0 min.')
  })

  it('shows an amber late badge with the delta', () => {
    expect(
      verdictBadge({ kind: 'checkpoint', onTime: false, deltaSeconds: 12 * 60, spentSeconds: null }),
    ).toEqual({ text: '12 min for sent', tone: 'warning' })
  })

  it('shows an amber early badge with the delta', () => {
    expect(
      verdictBadge({ kind: 'checkpoint', onTime: false, deltaSeconds: -3 * 60, spentSeconds: null }),
    ).toEqual({ text: '3 min for tidligt', tone: 'warning' })
  })

  it('renders no badge when the verdict is unknown', () => {
    // No window, no anchor, or unattributed: the BFF sends null, and absence is the honest rendering.
    expect(
      verdictBadge({ kind: 'checkpoint', onTime: null, deltaSeconds: null, spentSeconds: null }),
    ).toBeNull()
  })

  it('never badges a bandit catch', () => {
    // Bandit catches keep their own styling; a verdict on one would be meaningless.
    expect(
      verdictBadge({ kind: 'bandit', onTime: true, deltaSeconds: 0, spentSeconds: null }),
    ).toBeNull()
    expect(
      verdictBadge({ kind: 'bandit', onTime: null, deltaSeconds: null, spentSeconds: null }),
    ).toBeNull()
  })
})
