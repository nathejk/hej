import { describe, expect, it } from 'vitest'
import type { RouteLocationNormalized } from 'vue-router'

import { moderatorGate, roleGate } from '@/router/gates'
import { ALL_ROLES, type Role } from '@/config/roles'

// Step 7 of the router guard: the Glimt moderation gate (PRD 019 §6, task 309).
//
// # Why this is not `meta.roles`
//
// Moderation comes from the member's `sectionSlug`, not from their role. **Every Team member is
// `crew` as far as roles are concerned — and so are the kitchen, PR and every crew account whose
// section could not be classified.** Expressing this as `meta.roles: ['crew']` would therefore hand
// the widest read in the whole service (a queue with no visibility filter at all, containing every
// photograph in the event) to a large population of accounts, and it would look correct doing it.
// That is the mistake these tests exist to make impossible to reintroduce.
//
// Not the security boundary either. All three moderation endpoints re-read the caller's *current*
// assignment per request, so a caller who reaches the route anyway gets 403s and an empty screen.

function route(name: string, meta: Record<string, unknown> = {}) {
  return { name, meta } as unknown as RouteLocationNormalized
}

describe('moderatorGate', () => {
  it('ignores routes that are not gated on moderation', () => {
    for (const moderates of [true, false]) {
      expect(moderatorGate(route('glimt'), moderates)).toBe(true)
      expect(moderatorGate(route('maps'), moderates)).toBe(true)
    }
  })

  it('admits a moderator', () => {
    expect(moderatorGate(route('glimt-moderation', { moderator: true }), true)).toBe(true)
  })

  it('refuses everyone else, to the feed', () => {
    // The feed rather than the map: somebody following a stale moderation link was on their way to
    // Glimt, and landing there is a smaller surprise than being dropped on the map.
    expect(moderatorGate(route('glimt-moderation', { moderator: true }), false)).toEqual({
      name: 'glimt',
    })
  })

  // The inversion worth writing down: `roleGate` lets an unknown answer through, and this does not.
  //
  // For roles, falling through is right — bouncing a legitimate user off a page while the session
  // resolves is the blank-screen class of bug (task 090). Here the trade flips, because
  // `moderatesGlimt` is false both when the caller does not moderate *and* when we could not ask (an
  // offline cold start, where the remembered identity carries no such flag). Falling through would
  // then render a queue that cannot possibly load — the view needs the network by definition — so
  // refusing costs a moderator one reconnect and saves everyone else a screen they should not see.
  it('refuses when the answer is not known, unlike roleGate', () => {
    const gated = route('glimt-moderation', { moderator: true })
    expect(moderatorGate(gated, false)).toEqual({ name: 'glimt' })
    // The contrast, asserted rather than described.
    expect(roleGate(route('contacts', { roles: ['crew'] as Role[] }), null)).toBe(true)
  })

  // No role grants moderation. This is the whole point of the gate being separate.
  it('is not satisfied by any role', () => {
    const gated = route('glimt-moderation', { moderator: true })
    for (const role of ALL_ROLES) {
      // `roleGate` is the only thing a role can satisfy, and it has nothing to say here — so a
      // route gated only on `moderator` is refused for every role in the app.
      expect(roleGate(gated, role), `roleGate should not decide this for ${role}`).toBe(true)
      expect(moderatorGate(gated, false), `role ${role} must not imply moderation`).toEqual({
        name: 'glimt',
      })
    }
  })

  it('never throws', () => {
    // Nothing in the guard may reject or throw (task 090): that aborts the navigation and leaves a
    // blank white screen.
    for (const meta of [{}, { moderator: true }, { moderator: false }, { roles: ['crew'] }]) {
      for (const moderates of [true, false]) {
        expect(() => moderatorGate(route('x', meta), moderates)).not.toThrow()
      }
    }
  })
})
