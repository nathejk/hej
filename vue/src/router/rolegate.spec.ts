import { describe, expect, it } from 'vitest'
import type { RouteLocationNormalized } from 'vue-router'

import { roleGate } from '@/router/gates'
import { destinations } from '@/config/navigation'
import { ALL_ROLES, type Role } from '@/config/roles'

// Step 6 of the router guard: role gating (PRD 007 made it load-bearing rather than
// cosmetic — it is what refuses /contacts to a spejder who types the URL, follows a stale
// link, or restores a tab).
//
// Not the security boundary. The BFF answers 403 for a spejder on every contacts endpoint;
// this only stops the app rendering a pane it cannot fill.

function route(name: string, roles?: Role[]) {
  return { name, meta: roles ? { roles } : {} } as unknown as RouteLocationNormalized
}

// The contacts route as actually registered, so these tests break if the destination's gate
// changes — rather than testing a hand-written copy of it.
const contactsRoles = destinations.find((d) => d.name === 'contacts')!.roles

describe('roleGate', () => {
  it('allows an ungated route for every role', () => {
    for (const role of ALL_ROLES) {
      expect(roleGate(route('maps'), role)).toBe(true)
    }
    expect(roleGate(route('maps'), null)).toBe(true)
  })

  it('refuses /contacts to a spejder and redirects to the map', () => {
    const outcome = roleGate(route('contacts', contactsRoles), 'spejder')
    expect(outcome).not.toBe(true)
    // Somewhere sensible, never an error page.
    expect(outcome).toEqual({ name: 'maps' })
  })

  it('admits every non-spejder role to /contacts', () => {
    for (const role of ALL_ROLES.filter((r) => r !== 'spejder')) {
      expect(roleGate(route('contacts', contactsRoles), role), `role ${role}`).toBe(true)
    }
  })

  it('refuses the person profile route to a spejder too', () => {
    // The deep-linkable one: /contacts/:personId carries the same gate, so a shared link
    // cannot get a spejder into the pane sideways.
    const outcome = roleGate(route('contact-person', contactsRoles), 'spejder')
    expect(outcome).toEqual({ name: 'maps' })
  })

  // A null role means "the session has not resolved yet" — cold start, or offline. Falling
  // through is deliberate: redirecting then would bounce a legitimate user off a page they
  // are entitled to, and the endpoint still refuses, so the cost is an empty pane rather
  // than a disclosure.
  it('falls through when the role is not yet known', () => {
    expect(roleGate(route('contacts', contactsRoles), null)).toBe(true)
  })

  it('keeps refusing the SOS page to the roles it excludes', () => {
    // A regression guard: the SOS gate is an explicit allow-list, unlike contacts, and both
    // now flow through this one function.
    const sos = destinations.find((d) => d.name === 'sos')!
    expect(roleGate(route('sos', sos.roles), 'spejder')).toEqual({ name: 'maps' })
    expect(roleGate(route('sos', sos.roles), 'bandit')).toEqual({ name: 'maps' })
    expect(roleGate(route('sos', sos.roles), 'samarit')).toBe(true)
  })

  it('never throws for any role/route combination', () => {
    // Nothing in the guard may reject or throw (task 090): that aborts the navigation and
    // leaves a blank white screen.
    for (const d of destinations) {
      for (const role of [...ALL_ROLES, null]) {
        expect(() => roleGate(route(d.name, d.roles), role as Role | null)).not.toThrow()
      }
    }
  })
})
