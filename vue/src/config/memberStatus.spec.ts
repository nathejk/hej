import { describe, expect, it } from 'vitest'

import {
  MEMBER_STATUSES,
  hasEnded,
  hasLeftRace,
  isInOurCare,
  memberStatusLabel,
} from '@/config/memberStatus'

describe('memberStatusLabel', () => {
  it('has a Danish label for every known status', () => {
    for (const status of MEMBER_STATUSES) {
      const label = memberStatusLabel(status)
      expect(label, status).not.toBe('')
      // The raw value leaking through would mean a missing label.
      expect(label, status).not.toBe(status)
    }
  })

  it('returns an empty string for an empty status', () => {
    expect(memberStatusLabel('')).toBe('')
  })

  // A status this build does not know still means something to the crew member reading it, and
  // hiding it would silently under-report a member's situation during an incident.
  it('passes an unknown status through rather than hiding it', () => {
    expect(memberStatusLabel('airlifted')).toBe('airlifted')
  })
})

describe('hasLeftRace', () => {
  it('is true only for reunited and released', () => {
    for (const status of MEMBER_STATUSES) {
      const want = status === 'reunited' || status === 'released'
      expect(hasLeftRace(status), status).toBe(want)
    }
  })

  // The distinction shared-go's own docs go out of their way to protect: marking a finisher as a
  // withdrawal turns an achievement into a dropout.
  it('does not treat a finisher as having left the race', () => {
    expect(hasLeftRace('finished')).toBe(false)
  })

  // Assuming somebody has left the race on the strength of a value we do not recognise is the
  // more damaging guess.
  it('is false for an unknown status', () => {
    expect(hasLeftRace('airlifted')).toBe(false)
    expect(hasLeftRace('')).toBe(false)
  })
})

describe('hasEnded', () => {
  it('includes finished as well as the withdrawals', () => {
    expect(hasEnded('finished')).toBe(true)
    expect(hasEnded('reunited')).toBe(true)
    expect(hasEnded('released')).toBe(true)
    expect(hasEnded('racing')).toBe(false)
    expect(hasEnded('waiting')).toBe(false)
  })
})

describe('isInOurCare', () => {
  // Mirrors MemberStatus.InOurCare() in shared-go: from leaving the route to somebody else taking
  // charge. A samarit sent to a member who is already sheltered needs to know before setting off.
  it('covers waiting, transit and sheltered only', () => {
    for (const status of MEMBER_STATUSES) {
      const want = status === 'waiting' || status === 'transit' || status === 'sheltered'
      expect(isInOurCare(status), status).toBe(want)
    }
  })

  it('is false for a status we do not know', () => {
    expect(isInOurCare('airlifted')).toBe(false)
  })
})
