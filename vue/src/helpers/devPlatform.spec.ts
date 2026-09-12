import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { RouteLocationNormalized } from 'vue-router'

import type { DevDevice } from '@/config/devDevice'

// The profile is mocked rather than written to a fake storage, because what is under test here
// is the *mapping* from a profile to a simulated device — not the persistence, which
// devDevice.spec.ts already covers.
vi.mock('@/config/devDevice', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/config/devDevice')>()
  return { ...actual, readDevDevice: () => profile }
})

let profile: DevDevice | null = null

import { detectPlatform } from '@/config/permissions'
import { installPlatform, isMobileDevice, isStandalone } from '@/helpers/platform'

function sim(patch: Partial<DevDevice> = {}) {
  profile = { mobile: true, standalone: true, platform: 'ios', ...patch }
}

describe('dev device simulation, through the real predicates', () => {
  beforeEach(() => {
    profile = null
  })

  // The headline: a laptop reads as an installed phone, so the router gate runs steps 2-4 for
  // real instead of being switched off.
  it('makes a simulated iPhone read as mobile and standalone', () => {
    sim({ platform: 'ios' })
    expect(isMobileDevice()).toBe(true)
    expect(isStandalone()).toBe(true)
    expect(installPlatform()).toBe('ios-safari')
    expect(detectPlatform()).toBe('ios')
  })

  // The install wall's only route onto a laptop.
  it('tab-style profiles read as mobile but not standalone', () => {
    sim({ standalone: false })
    expect(isMobileDevice()).toBe(true)
    expect(isStandalone()).toBe(false)
  })

  // Simulated through the *real* heuristic: a MacIntel navigator with touch points. If
  // `isAppleTouchDevice` were wrong, this would be wrong too — which is the intent, since a
  // simulation that answered correctly by construction would hide the bug.
  it('classifies a simulated iPad as mobile via the MacIntel + touch path', () => {
    sim({ platform: 'ipad' })
    expect(isMobileDevice()).toBe(true)
    expect(installPlatform()).toBe('ios-safari')
    expect(detectPlatform()).toBe('ios')
  })

  it('maps android and chromium onto the Chromium install instructions', () => {
    sim({ platform: 'android' })
    expect(installPlatform()).toBe('chromium')
    expect(detectPlatform()).toBe('android')

    sim({ platform: 'chromium' })
    expect(installPlatform()).toBe('chromium')
  })

  // Android has no navigator.standalone, so display-mode is what must answer. Setting both
  // would let the simulation pass isStandalone() by a route no real Android device uses.
  it('reaches standalone on android through display-mode, not navigator.standalone', () => {
    sim({ platform: 'android', standalone: true })
    expect(isStandalone()).toBe(true)
    sim({ platform: 'android', standalone: false })
    expect(isStandalone()).toBe(false)
  })

  it('reaches the webview branch, which is never standalone', () => {
    sim({ platform: 'webview', standalone: false })
    expect(installPlatform()).toBe('webview')
    expect(isMobileDevice()).toBe(true)
    expect(isStandalone()).toBe(false)
  })

  // The generic guidance copy is otherwise unreachable from any simulated device: every other
  // preset is either iOS or Android.
  it('reaches the generic install and guidance branches', () => {
    sim({ platform: 'other' })
    expect(installPlatform()).toBe('other')
    expect(detectPlatform()).toBe('other')
  })

  // A profile can only say this if something wrote it directly; no preset produces it. The
  // stored value is input, so the synthesis has to have an answer for it.
  it('honours a mobile: false profile as a desktop computer', () => {
    sim({ mobile: false, standalone: false })
    expect(isMobileDevice()).toBe(false)
  })

  // No profile: the real environment answers. In vitest that is a node global with no
  // navigator, so the assertion is only that nothing throws and nothing claims to be a phone.
  it('falls back to the real environment when no profile is set', () => {
    profile = null
    expect(() => detectPlatform()).not.toThrow()
    expect(detectPlatform()).toBe('other')
  })
})

// The predicates being right is necessary but not the claim PRD 014 makes. The claim is that
// the **gate chain runs for real** against a simulated device — as opposed to `?nogate=1`,
// which reaches the same routes by switching steps 2-4 off, onboarding redirect included.
//
// So this drives the actual gate with the actual platform helpers. `gates.spec.ts` mocks them
// (it is testing the decision tree), which means nothing else in the suite would notice if the
// simulation and the gate disagreed.
describe('the gate chain, against a simulated device', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    profile = null
  })

  function route(name: string) {
    return { name, meta: {} } as unknown as RouteLocationNormalized
  }

  it('sends a simulated installed phone into onboarding, not past it', async () => {
    sim({ platform: 'ios', standalone: true })
    const { deviceAndInstallGates } = await import('@/router/gates')

    // The point: onboarding is incomplete, so the gate redirects. A bypass would have
    // returned `true` here and dropped the developer straight into the app.
    expect(deviceAndInstallGates(route('maps'))).toEqual({ name: 'welcome' })
  })

  it('sends a simulated browser tab to the install wall', async () => {
    sim({ platform: 'ios', standalone: false })
    const { deviceAndInstallGates } = await import('@/router/gates')

    expect(deviceAndInstallGates(route('maps'))).toEqual({ name: 'install' })
    expect(deviceAndInstallGates(route('install'))).toBe(true)
  })

  it('leaves the app when the profile is cleared — real detection is restored', async () => {
    profile = null
    // The real `defaultEnv()` calls `globalThis.matchMedia`, which node does not have — which
    // is why `gates.spec.ts` mocks the platform helpers and why the router wraps the guard in
    // try/catch (task 090). Here a mouse-only desktop is stubbed in, because the assertion is
    // about detection being *real* again, not about node.
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    const { LEAVE_APP, deviceAndInstallGates } = await import('@/router/gates')

    expect(deviceAndInstallGates(route('maps'))).toBe(LEAVE_APP)
    vi.unstubAllGlobals()
  })
})
