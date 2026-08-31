import { describe, it, expect } from 'vitest'

import { destinations, visibleDestinations } from '@/config/navigation'
import { ALL_ROLES, allRolesExcept, isCrewRole, type Role } from '@/config/roles'

// PRD 007: spejdere do not get the contacts pane, in either direction — they see nobody and
// appear to nobody. These tests pin the nav half of that.
//
// Worth saying plainly, because a test file like this can be mistaken for the security
// boundary: it is not. The BFF answers 403, and the router guard refuses the route. Hiding
// the nav entry only stops a spejder being shown a door that will not open.

const contacts = () => destinations.find((d) => d.name === 'contacts')!

describe('contacts destination', () => {
  it('exists and is not fullBleed', () => {
    const d = contacts()
    expect(d).toBeDefined()
    // The list scrolls, and fullBleed drops App.vue's overflow-y-auto wrapper.
    expect(d.fullBleed).toBeUndefined()
  })

  it('is hidden from spejdere', () => {
    const visible = visibleDestinations('spejder').map((d) => d.name)
    expect(visible).not.toContain('contacts')
  })

  it('is visible to every other role', () => {
    for (const role of ALL_ROLES.filter((r) => r !== 'spejder')) {
      const visible = visibleDestinations(role).map((d) => d.name)
      expect(visible, `role ${role} should see the contacts pane`).toContain('contacts')
    }
  })

  // The gate is "everyone except spejder", not a list of the six roles that exist today. A
  // role added to ALL_ROLES should get the pane by default rather than being silently left
  // out of it — the failure mode of an allow-list nobody remembers to update.
  it('admits a newly added role by default', () => {
    const permitted = contacts().roles!
    const missing = ALL_ROLES.filter((r) => r !== 'spejder' && !permitted.includes(r))
    expect(missing, 'roles missing from the contacts gate').toEqual([])
    expect(permitted).toEqual(allRolesExcept('spejder'))
  })

  it('does not accidentally gate the shared content pages', () => {
    // A regression guard for the obvious mistake while editing this file: gating something
    // that should stay open to everyone, including spejdere.
    for (const name of ['maps', 'rulebook', 'updates', 'schedule', 'faq', 'privacy']) {
      const d = destinations.find((x) => x.name === name)!
      expect(d.roles, `${name} must stay open to every role`).toBeUndefined()
    }
  })
})

describe('isCrewRole', () => {
  // Mirrors Role.IsCrew() in go/internal/users/directory.go. Decides whether the patrol lookup's
  // entry point is drawn — the BFF enforces the actual permission.
  it('is true for the crew functions and the fallback', () => {
    for (const role of ['postmandskab', 'guide', 'samarit', 'crew'] as Role[]) {
      expect(isCrewRole(role), role).toBe(true)
    }
  })

  it('is false for participants', () => {
    for (const role of ['spejder', 'bandit', 'gøgler'] as Role[]) {
      expect(isCrewRole(role), role).toBe(false)
    }
  })

  // PRD 007 §11.3: including the fallback is a decision, not an accident — task 078 found every
  // real crew member currently lands on it, so excluding it would ship a feature with no users.
  it('includes the unclassified crew fallback deliberately', () => {
    expect(isCrewRole('crew')).toBe(true)
  })

  it('is false when the role is not yet known', () => {
    expect(isCrewRole(null)).toBe(false)
  })

  it('covers every role in ALL_ROLES', () => {
    // Guards against a role being added and silently landing on the wrong side.
    const crew = ALL_ROLES.filter((r) => isCrewRole(r))
    expect(crew).toEqual(['postmandskab', 'guide', 'samarit', 'crew'])
  })
})

describe('allRolesExcept', () => {
  it('returns every role but the excluded ones', () => {
    expect(allRolesExcept('spejder')).not.toContain('spejder')
    expect(allRolesExcept('spejder')).toHaveLength(ALL_ROLES.length - 1)
  })

  it('accepts several exclusions', () => {
    const got = allRolesExcept('spejder', 'bandit')
    expect(got).not.toContain('spejder')
    expect(got).not.toContain('bandit')
    expect(got).toHaveLength(ALL_ROLES.length - 2)
  })

  it('preserves the declared order', () => {
    // Order drives the bottom nav, so it is not incidental.
    const got = allRolesExcept('bandit')
    const want = ALL_ROLES.filter((r: Role) => r !== 'bandit')
    expect(got).toEqual(want)
  })

  it('returns everything when nothing is excluded', () => {
    expect(allRolesExcept()).toEqual([...ALL_ROLES])
  })
})
