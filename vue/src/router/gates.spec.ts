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

import { LEAVE_APP, WEBSITE_PAGE, deviceAndInstallGates } from '@/router/gates'
import { useOnboardingStore } from '@/stores/onboarding.store'

// A minimal stand-in for what the guard actually reads off a route.
function route(
  name: string | undefined,
  meta: Record<string, unknown> = {},
  extra: Record<string, unknown> = {},
) {
  return { name, meta, path: `/${name ?? ''}`, ...extra } as unknown as RouteLocationNormalized
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

  // Task 356. The public website is for **every** device; only the app's own pages are install-only.
  // `/` is what people type and what gets shared, so answering it with an add-to-home-screen wall
  // pushes the app at a visitor who came to read the site.
  //
  // The root reaches the guard as a redirect to `maps`, which is exactly why this is easy to get
  // wrong: the destination looks like any other app page unless `redirectedFrom` is consulted.
  it('sends a mobile browser that arrived at the root to the website, not the wall', () => {
    standalone = false
    const fromRoot = route('maps', {}, { redirectedFrom: { path: '/' } })
    expect(deviceAndInstallGates(fromRoot)).toBe(LEAVE_APP)

    // And the same if the root ever becomes a route of its own rather than a redirect.
    expect(deviceAndInstallGates(route(undefined, {}, { path: '/' }))).toBe(LEAVE_APP)
  })

  // The other half of the same rule: asking for an app page by name is not arriving at the front
  // door. Those pages are install-only, so the wall is the honest answer.
  it('still walls an app page a browser asked for by name', () => {
    standalone = false
    for (const name of ['maps', 'sos', 'profile', 'glimt']) {
      expect(deviceAndInstallGates(route(name))).toEqual({ name: 'install' })
    }
  })

  // An installed launch also comes through `/` — start_url is the root — so the root exception must
  // not swallow it. It is inside the non-standalone branch precisely for this reason.
  it('does not send an installed launch at the root out to the website', () => {
    standalone = true
    useOnboardingStore().markComplete()
    const fromRoot = route('maps', {}, { redirectedFrom: { path: '/' } })
    expect(deviceAndInstallGates(fromRoot)).toBe(true)
  })

  it('leaves the app entirely on a desktop computer', () => {
    mobile = false
    expect(settle('maps', { authenticated: true })).toBe('(left the app)')
  })

  // **Where it leaves to is the public site** (task 351). `/desktop.html` said "more to come…" while PRD
  // 011's real pages sat one directory away, so a desktop visitor was being left at a dead end.
  //
  // The path is derived from the calendar year, which is the same default the server uses — there is no
  // alias to ask instead, and no config to wait for at this point in the boot. A mismatch (somebody
  // serving a past event via EVENT_YEAR) lands on the public site's own not-found page, which links
  // onward; it cannot bounce back into the app, because every year-shaped path is the public site's.
  it('leaves for the public site under the event-year prefix', () => {
    expect(WEBSITE_PAGE).toMatch(/^\/\d{4}$/)
    expect(WEBSITE_PAGE).toBe(`/${new Date().getFullYear()}`)
    expect(WEBSITE_PAGE).not.toContain('.html')
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
