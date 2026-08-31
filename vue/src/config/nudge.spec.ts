import { describe, expect, it } from 'vitest'

import { showPortraitNudge, type PortraitNudgeContext } from '@/config/nudge'

// A member who has been through onboarding without taking a photo, sitting on an ordinary page.
const nudgeable: PortraitNudgeContext = {
  hasPhoto: false,
  profileLoaded: true,
  dismissed: false,
  routeName: 'rulebook',
  fullBleed: false,
}

describe('showPortraitNudge', () => {
  it('nudges a member with no portrait', () => {
    expect(showPortraitNudge(nudgeable)).toBe(true)
  })

  // The point of driving it from `hasPhoto` rather than a stored flag: it cannot get stuck on for
  // someone who has already done what it asks.
  it('never nudges once a portrait exists', () => {
    expect(showPortraitNudge({ ...nudgeable, hasPhoto: true })).toBe(false)
  })

  // Without this guard the nudge flashes on every cold start before GET /api/me/profile resolves,
  // because hasPhoto defaults to false — i.e. it would appear most reliably for the members who
  // have *already* complied, which is the fastest way to teach everyone to dismiss it on sight.
  it('waits for the profile before deciding', () => {
    expect(showPortraitNudge({ ...nudgeable, profileLoaded: false })).toBe(false)
  })

  it('respects a dismissal', () => {
    expect(showPortraitNudge({ ...nudgeable, dismissed: true })).toBe(false)
  })

  // The map. Excluded structurally rather than by name: it is the one full-bleed route, it is an
  // operational surface during the race, and it already carries the location pre-prompt and the
  // offline notice.
  it('stays off full-bleed routes', () => {
    expect(showPortraitNudge({ ...nudgeable, routeName: 'maps', fullBleed: true })).toBe(false)
  })

  // An emergency page: nothing may compete for attention, and a member opening it is not in a
  // position to take a selfie.
  it('stays off the SOS page', () => {
    expect(showPortraitNudge({ ...nudgeable, routeName: 'sos' })).toBe(false)
  })

  // The real photo control is already on that page; a banner above it asking for a photo would be
  // noise next to the affordance it is pointing at.
  it('stays off the profile page', () => {
    expect(showPortraitNudge({ ...nudgeable, routeName: 'profile' })).toBe(false)
  })

  it('shows on the ordinary content routes', () => {
    for (const routeName of ['contacts', 'rulebook', 'updates', 'schedule', 'faq']) {
      expect(showPortraitNudge({ ...nudgeable, routeName }), routeName).toBe(true)
    }
  })

  // A member already racing without a portrait is precisely the person PRD 005 §11 says needs one
  // most, so nothing here keys off lifecycle state — there is deliberately no "the event has
  // started, stop asking" branch to find.
  it('has no lifecycle condition to switch it off', () => {
    expect(showPortraitNudge({ ...nudgeable, routeName: 'updates' })).toBe(true)
  })
})
