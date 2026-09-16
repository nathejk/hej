import { describe, it, expect } from 'vitest'
import { deltaText, verdictBadge } from './scanVerdict'

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

describe('verdictBadge', () => {
  it('shows a green "på tid" badge on time', () => {
    expect(verdictBadge({ kind: 'checkpoint', onTime: true, deltaSeconds: 0 })).toEqual({
      text: 'på tid',
      tone: 'success',
    })
  })

  it('shows an amber late badge with the delta', () => {
    expect(verdictBadge({ kind: 'checkpoint', onTime: false, deltaSeconds: 12 * 60 })).toEqual({
      text: '12 min for sent',
      tone: 'warning',
    })
  })

  it('shows an amber early badge with the delta', () => {
    expect(verdictBadge({ kind: 'checkpoint', onTime: false, deltaSeconds: -3 * 60 })).toEqual({
      text: '3 min for tidligt',
      tone: 'warning',
    })
  })

  it('renders no badge when the verdict is unknown', () => {
    // No window, no anchor, or unattributed: the BFF sends null, and absence is the honest rendering.
    expect(verdictBadge({ kind: 'checkpoint', onTime: null, deltaSeconds: null })).toBeNull()
  })

  it('never badges a bandit catch', () => {
    // Bandit catches keep their own styling; a verdict on one would be meaningless.
    expect(verdictBadge({ kind: 'bandit', onTime: true, deltaSeconds: 0 })).toBeNull()
    expect(verdictBadge({ kind: 'bandit', onTime: null, deltaSeconds: null })).toBeNull()
  })
})
