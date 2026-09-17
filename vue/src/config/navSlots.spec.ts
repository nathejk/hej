import { describe, expect, it } from 'vitest'

import { ALL_ROLES, type Role } from '@/config/roles'
import { destinations, visibleDestinations } from '@/config/navigation'
import { MAX_SLOTS, inBar, splitNavSlots } from '@/config/navSlots'

// The bottom-bar slot allocation, per role (task 320, PRD 019 §7, §11 Q7).
//
// # Why this is a test and not a comment
//
// `navigation.ts` is one ordered array that gets filtered by role, so the bar a given role sees is an
// *emergent* property of the list order and the role gates together. Nobody can read the array and be
// sure a spejder still has Glimt in the bar — least of all the person adding the eleventh destination.
// These tests turn that reading into a failure.
//
// They are deliberately about **outcomes** ("a spejder has Glimt in the bar"), not about the array's
// contents, so a future re-ordering that preserves the requirement is free, and one that breaks it is
// not.

function slotsFor(role: Role) {
  return splitNavSlots(visibleDestinations(role))
}

describe('nav slot allocation', () => {
  it('spends the last slot on the burger, not on a destination', () => {
    // The off-by-one that would silently drop a destination from both lists.
    const many = destinations
    expect(many.length).toBeGreaterThan(MAX_SLOTS)
    const { primary, overflow, hasOverflow } = splitNavSlots(many)
    expect(hasOverflow).toBe(true)
    expect(primary).toHaveLength(MAX_SLOTS - 1)
    expect(primary.length + overflow.length).toBe(many.length)
  })

  it('does not overflow when everything fits', () => {
    const four = destinations.slice(0, 4)
    const { primary, overflow, hasOverflow } = splitNavSlots(four)
    expect(hasOverflow).toBe(false)
    expect(primary).toEqual(four)
    expect(overflow).toEqual([])
  })

  it('never renders more than MAX_SLOTS tappable slots for any role', () => {
    for (const role of ALL_ROLES) {
      const { primary, hasOverflow } = slotsFor(role)
      // The burger is itself a slot when present.
      expect(primary.length + (hasOverflow ? 1 : 0), role).toBeLessThanOrEqual(MAX_SLOTS)
    }
  })

  // The invariant the task calls "nothing becomes unreachable". Reachability is the thing that can
  // actually hurt a user: a destination in neither list is a page with no way in.
  it('leaves every visible destination reachable for every role', () => {
    for (const role of ALL_ROLES) {
      const visible = visibleDestinations(role)
      const { primary, overflow } = slotsFor(role)
      expect([...primary, ...overflow].map((d) => d.name), role).toEqual(visible.map((d) => d.name))
    }
  })

  // PRD 019 §7: Glimt is a *primary* destination for the participants, so it must not be behind
  // "Mere" for them. This is the requirement the ordering exists to satisfy.
  it('puts Glimt in the bar for spejder and bandit', () => {
    for (const role of ['spejder', 'bandit'] as Role[]) {
      expect(inBar(visibleDestinations(role), 'glimt'), role).toBe(true)
    }
  })

  // Stronger than the above and the one most likely to catch a regression: every role currently gets
  // Glimt in the bar, which is what makes the single shared ordering defensible. If a future
  // destination has to displace it, let that be a deliberate edit to this list rather than a silent
  // change of the feature's prominence.
  it('puts Glimt in the bar for every role', () => {
    for (const role of ALL_ROLES) {
      expect(inBar(visibleDestinations(role), 'glimt'), role).toBe(true)
    }
  })

  it('keeps the map first for every role', () => {
    for (const role of ALL_ROLES) {
      expect(slotsFor(role).primary[0]?.name, role).toBe('maps')
    }
  })

  // Not a slot rule but the rule that protects the slots: organizer tooling does not get one.
  // Moderation (task 309) is for a handful of Team-section accounts and would cost every
  // participant a slot to reach a page they may not open.
  it('has no moderation destination', () => {
    expect(destinations.map((d) => d.name)).not.toContain('glimt-moderation')
    expect(destinations.map((d) => d.path)).not.toContain('/glimt/moderation')
  })

  // A spejder has no contacts pane (PRD 007) and so no directory of minors' faces. Asserted here
  // because the *reason* Glimt reaches third position in their bar is that this entry is absent —
  // if it ever appears for them, this file's arithmetic is the least of the problems.
  it('gives a spejder no contacts entry', () => {
    expect(visibleDestinations('spejder').map((d) => d.name)).not.toContain('contacts')
  })
})
