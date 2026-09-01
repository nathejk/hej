import { describe, expect, it } from 'vitest'

import { candidateRows, candidateSubtitle } from '@/helpers/profileCandidates'
import type { ChoiceCandidate } from '@/stores/session.store'

// Reported 2026-09-01 with a screenshot: five rows all reading "Klaus". The number carried one
// `postmandskab` profile named "Klaus Jørgensen" and four `gøgler` profiles named "Klaus", and the
// payload discarded every discriminator the data had — the surname was stripped and the role was not
// sent at all.
//
// These pin the display rules that make such a list answerable.

function candidate(over: Partial<ChoiceCandidate> = {}): ChoiceCandidate {
  return { user_id: 'a', name: 'Klaus', ...over }
}

describe('candidateSubtitle', () => {
  it('prefers the team, which is what separates two siblings', () => {
    expect(candidateSubtitle(candidate({ team: 'Klan Ravn', section: 'Ignored', role: 'bandit' }))).toBe(
      'Klan Ravn',
    )
  })

  it('falls back to the section', () => {
    expect(candidateSubtitle(candidate({ section: 'Samaritter', role: 'samarit' }))).toBe('Samaritter')
  })

  // The case the real duplicates produce: no affiliation at all, so the role is the only difference.
  it('falls back to the role label', () => {
    expect(candidateSubtitle(candidate({ role: 'gøgler' }))).toBe('Gøgler')
    expect(candidateSubtitle(candidate({ role: 'postmandskab' }))).toBe('Postmandskab')
  })

  it('passes an unknown role through rather than hiding it', () => {
    // A role this build does not know still tells the reader something, and a blank line tells them
    // nothing.
    expect(candidateSubtitle(candidate({ role: 'kaptajn' }))).toBe('kaptajn')
  })

  it('is empty when the record offers nothing', () => {
    expect(candidateSubtitle(candidate())).toBe('')
  })
})

describe('candidateRows', () => {
  it('leaves distinguishable rows alone', () => {
    const rows = candidateRows([
      candidate({ user_id: 'a', name: 'Klaus Jørgensen', section: 'Postmandskab', role: 'postmandskab' }),
      candidate({ user_id: 'b', name: 'Klaus', role: 'gøgler' }),
    ])

    expect(rows[0]).toEqual({
      userId: 'a',
      name: 'Klaus Jørgensen',
      subtitle: 'Postmandskab',
    })
    expect(rows[1].subtitle).toBe('Gøgler')
    expect(rows.map((r) => r.subtitle).join(' ')).not.toContain('profil')
  })

  // Nine rows for one member exists in the real data. Nothing distinguishes them, so numbering is
  // the honest answer: it says "these are distinct records" and lets somebody pick deliberately
  // rather than guessing which of five identical lines they already tried.
  it('numbers rows that are otherwise identical', () => {
    const rows = candidateRows([
      candidate({ user_id: 'a', role: 'gøgler' }),
      candidate({ user_id: 'b', role: 'gøgler' }),
      candidate({ user_id: 'c', role: 'gøgler' }),
    ])

    expect(rows.map((r) => r.subtitle)).toEqual([
      'Gøgler · profil 1',
      'Gøgler · profil 2',
      'Gøgler · profil 3',
    ])
  })

  // The mixed case from the screenshot. Only the ambiguous rows are numbered, so a number never
  // implies an ordering over the whole list.
  it('numbers only the ambiguous rows', () => {
    const rows = candidateRows([
      candidate({ user_id: 'a', name: 'Klaus Jørgensen', section: 'Postmandskab', role: 'postmandskab' }),
      candidate({ user_id: 'b', role: 'gøgler' }),
      candidate({ user_id: 'c', role: 'gøgler' }),
    ])

    expect(rows[0].subtitle).toBe('Postmandskab')
    expect(rows[1].subtitle).toBe('Gøgler · profil 1')
    expect(rows[2].subtitle).toBe('Gøgler · profil 2')
  })

  it('numbers identical rows that do have an affiliation', () => {
    // Cæcilie: nine rows, same patrol, same role — the largest cluster in the real data.
    const rows = candidateRows([
      candidate({ user_id: 'a', name: 'Cæcilie', team: 'Ging gang goolie', role: 'spejder' }),
      candidate({ user_id: 'b', name: 'Cæcilie', team: 'Ging gang goolie', role: 'spejder' }),
    ])

    expect(rows[0].subtitle).toBe('Ging gang goolie · profil 1')
    expect(rows[1].subtitle).toBe('Ging gang goolie · profil 2')
  })

  it('numbers rows with no discriminator at all, without a leading separator', () => {
    const rows = candidateRows([candidate({ user_id: 'a' }), candidate({ user_id: 'b' })])

    expect(rows[0].subtitle).toBe('profil 1')
    expect(rows[1].subtitle).toBe('profil 2')
  })

  it('keeps the user id, which is what the caller acts on', () => {
    const rows = candidateRows([candidate({ user_id: 'the-id' })])
    expect(rows[0].userId).toBe('the-id')
  })

  it('preserves order and length', () => {
    const input = [
      candidate({ user_id: 'a', name: 'A' }),
      candidate({ user_id: 'b', name: 'B' }),
      candidate({ user_id: 'c', name: 'C' }),
    ]
    expect(candidateRows(input).map((r) => r.userId)).toEqual(['a', 'b', 'c'])
  })

  it('handles an empty list', () => {
    expect(candidateRows([])).toEqual([])
  })
})
