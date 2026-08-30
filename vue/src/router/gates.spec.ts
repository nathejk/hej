import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { RouteLocationNormalized, RouteLocationRaw } from 'vue-router'

// Mocked because the real helpers read `navigator`/`matchMedia`, which is the entire reason
// they take an injectable environment — but the guard calls them with the defaults.
vi.mock('@/helpers/platform', () => ({
  isMobileDevice: () => mobile,
  isStandalone: () => standalone,
}))

let mobile = true
let standalone = true

import { LEAVE_APP, deviceAndInstallGates } from '@/router/gates'
import { useOnboardingStore } from '@/stores/onboarding.store'

// A minimal stand-in for what the guard actually reads off a route.
function route(name: string | undefined, meta: Record<string, unknown> = {}) {
  return { name, meta } as unknown as RouteLocationNormalized
}

// The public routes, as registered. `welcome` and `install` are public; app routes are not.
const PUBLIC = new Set(['welcome', 'install'])

/**
 * Follows the gate **and** the auth redirect until nothing more changes, or gives up.
 *
 * This models the whole chain rather than one function, because that is where the bug lived:
 * the gate and the auth fallback were each individually sensible and together formed a cycle.
 * vue-router aborts an infinite redirect, which renders nothing at all — the user sees a page
 * that never finishes loading, with no error in the UI.
 */
function settle(start: string, opts: { authenticated: boolean }): string {
  let current = start
  for (let hops = 0; hops < 10; hops += 1) {
    const to = route(current, { public: PUBLIC.has(current) })

    const gated = deviceAndInstallGates(to)
    if (gated === LEAVE_APP) return '(left the app)'
    if (gated !== true) {
      const next = (gated as { name: string }).name
      if (next === current) return current
      current = next
      continue
    }

    // The auth step of the real guard.
    if (!PUBLIC.has(current) && !opts.authenticated) {
      if (current === 'welcome') return current
      current = 'welcome'
      continue
    }
    return current
  }
  throw new Error(`infinite redirect starting at ${start}`)
}

describe('device / install / onboarding gates', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mobile = true
    standalone = true
  })

  it('sends a mobile browser tab to the install wall, and the wall renders there', () => {
    standalone = false
    expect(settle('maps', { authenticated: true })).toBe('install')
    expect(settle('install', { authenticated: false })).toBe('install')
  })

  it('leaves the app entirely on a desktop computer', () => {
    mobile = false
    expect(settle('maps', { authenticated: true })).toBe('(left the app)')
  })

  // There is no way past the wall in a browser any more (task 143): the website is anonymous
  // and login exists only in the installed app. So no state, and no URL, gets a tab to the
  // login flow — including asking for /welcome by hand.
  it('never lets a browser tab reach the login flow, whatever it asks for', () => {
    standalone = false
    for (const start of ['maps', 'welcome', 'profile', 'sos']) {
      expect(settle(start, { authenticated: false })).toBe('install')
      expect(settle(start, { authenticated: true })).toBe('install')
    }
  })

  it('sends an installed device with onboarding unfinished to /welcome', () => {
    expect(settle('maps', { authenticated: true })).toBe('welcome')
  })

  it('lets an onboarded, authenticated user reach the app', () => {
    useOnboardingStore().markComplete()
    expect(settle('maps', { authenticated: true })).toBe('maps')
  })

  // THE REGRESSION. An onboarded device whose session has expired used to go
  // maps → welcome → maps → … forever: the app never rendered and the page never finished
  // loading. A 7-day session plus a per-device completion flag makes this the ordinary state
  // of anyone returning a week later, so it bricked the app rather than inconveniencing it.
  it('does not loop for an onboarded device whose session has expired', () => {
    useOnboardingStore().markComplete()
    expect(settle('maps', { authenticated: false })).toBe('welcome')
    expect(settle('welcome', { authenticated: false })).toBe('welcome')
  })

  // The exhaustive version of the above: every combination has to reach a fixpoint. `settle`
  // throws on a cycle, so this failing is the loop, not a wrong destination.
  it('terminates for every combination of device, install, onboarding and session state', () => {
    for (const isMobile of [true, false]) {
      for (const isStandalone of [true, false]) {
        for (const complete of [true, false]) {
          for (const authenticated of [true, false]) {
            for (const start of ['maps', 'welcome', 'install', 'profile', 'sos']) {
              setActivePinia(createPinia())
              mobile = isMobile
              standalone = isStandalone
              if (complete) useOnboardingStore().markComplete()

              expect(() => settle(start, { authenticated }),
                `start=${start} mobile=${isMobile} standalone=${isStandalone} ` +
                  `complete=${complete} auth=${authenticated}`,
              ).not.toThrow()
            }
          }
        }
      }
    }
  })
})
