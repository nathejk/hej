import { describe, it, expect } from 'vitest'
import { scanVariant } from './scanAppearance'

describe('scanVariant', () => {
  it('classifies a bandit catch', () => {
    expect(scanVariant({ kind: 'bandit', checkpointId: '' })).toBe('bandit')
  })

  it('classifies a scan attributed to a post', () => {
    expect(scanVariant({ kind: 'checkpoint', checkpointId: 'cp-1' })).toBe('checkpoint')
  })

  // The case this function exists for. The BFF reports every non-bandit scan as `kind: 'checkpoint'`,
  // because that is the honest default when the scanner cannot be classified — so branching on `kind` alone
  // would give a flag and emerald emphasis to a registration we cannot place anywhere.
  it('classifies an unattributed registration as plain, despite its kind', () => {
    expect(scanVariant({ kind: 'checkpoint', checkpointId: '' })).toBe('plain')
  })

  // A bandit catch has no checkpoint either, and must not fall through to plain.
  it('prefers bandit over plain when neither has a checkpoint', () => {
    expect(scanVariant({ kind: 'bandit', checkpointId: '' })).toBe('bandit')
  })

  it('only ever returns one of the three known variants', () => {
    const cases = [
      { kind: 'bandit', checkpointId: 'cp-1' },
      { kind: 'bandit', checkpointId: '' },
      { kind: 'checkpoint', checkpointId: 'cp-1' },
      { kind: 'checkpoint', checkpointId: '' },
    ] as const

    for (const c of cases) {
      expect(['bandit', 'checkpoint', 'plain']).toContain(scanVariant(c))
    }
  })
})
