import type { PlatformEnv, PlatformNavigator } from '@/helpers/platform'

import type { DevDevice, DevPlatform } from '@/dev/devDevice'

// Turns a dev device profile into a `PlatformEnv` the real detection helpers can be fed.
//
// This module lives under `src/dev/` and is reachable only through `@/dev/bootstrap`, which
// `main.ts` dynamic-imports behind `import.meta.env.DEV`. That placement is load-bearing
// rather than tidy-minded: when `@/helpers/platform` imported this table directly, the fake
// user-agent strings below ended up in `dist/assets/index-*.js` of a production build. An
// `import.meta.env` guard hides code; it does not remove code something still imports.

// One plausible navigator per simulated platform.
//
// These are **real user-agent strings**, not tokens invented to satisfy the checks in
// `@/helpers/platform`, and that is the point: the simulation is fed through the same
// heuristics a real device is, so `?dev=ipad` exercises the `MacIntel` + touch-points path
// rather than bypassing it. If a heuristic is wrong, simulating it should reproduce the
// wrongness — a simulation that answered correctly by construction would hide exactly the bug
// the device matrix in task 139 exists to find.
//
// `apple` is carried explicitly rather than re-derived, because it decides whether
// `navigator.standalone` is set (see below) and that is a property of the *platform*, not
// something worth re-sniffing from a string this file just wrote.
interface Simulated {
  navigator: Omit<PlatformNavigator, 'standalone'>
  apple: boolean
}

const SIMULATED: Record<DevPlatform, Simulated> = {
  ios: {
    apple: true,
    navigator: {
      userAgent:
        'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1',
      platform: 'iPhone',
      maxTouchPoints: 5,
    },
  },
  // The awkward one, and the reason it is worth simulating at all: iPadOS 13+ requests desktop
  // sites, so the UA says Macintosh and only the touch points give it away.
  ipad: {
    apple: true,
    navigator: {
      userAgent:
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15',
      platform: 'MacIntel',
      maxTouchPoints: 5,
    },
  },
  android: {
    apple: false,
    navigator: {
      userAgent:
        'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36',
      maxTouchPoints: 5,
      userAgentData: { mobile: true, brands: [{ brand: 'Google Chrome' }, { brand: 'Chromium' }] },
    },
  },
  // Deliberately the same device as `android`. `chromium` is what `installPlatform()` returns,
  // so it is the word a developer reaches for when they want to see that branch; making it an
  // alias is cheaper than making them remember which vocabulary applies where.
  chromium: {
    apple: false,
    navigator: {
      userAgent:
        'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36',
      maxTouchPoints: 5,
      userAgentData: { mobile: true, brands: [{ brand: 'Google Chrome' }, { brand: 'Chromium' }] },
    },
  },
  // Facebook's in-app browser on iOS — the case participants arriving from a Facebook group
  // actually hit, where installation is impossible and the wall has to say so.
  webview: {
    apple: true,
    navigator: {
      userAgent:
        'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/468.0.0.32.107]',
      platform: 'iPhone',
      maxTouchPoints: 5,
    },
  },
  // A mobile browser that is neither WebKit-on-iOS nor Chromium: Firefox. No `Android` token,
  // so it also reaches the generic branch of `detectPlatform()` in @/config/permissions — the
  // one whose copy is otherwise unreachable from any simulated device.
  other: {
    apple: false,
    navigator: {
      userAgent: 'Mozilla/5.0 (Mobile; rv:127.0) Gecko/127.0 Firefox/127.0',
      maxTouchPoints: 5,
    },
  },
}

// A desktop computer, for the (write-only) case of a profile that says `mobile: false`. No
// preset produces one — `?dev=desktop` clears the profile instead, which is better because it
// restores *real* detection — but the profile is a stored value and this module does not get to
// assume which shapes reach it.
const SIMULATED_DESKTOP: Simulated = {
  apple: false,
  navigator: {
    userAgent:
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36',
    platform: 'MacIntel',
    maxTouchPoints: 0,
    userAgentData: { mobile: false, brands: [{ brand: 'Google Chrome' }] },
  },
}

export function simulatedNavigator(profile: DevDevice): PlatformNavigator {
  const sim = profile.mobile ? SIMULATED[profile.platform] : SIMULATED_DESKTOP
  return {
    ...sim.navigator,
    // WebKit-only, so it is set only for the Apple devices. Elsewhere the display-mode query
    // below is what answers, which is also how a real Android device behaves — setting both
    // would make the simulation pass `isStandalone()` for a reason no real device uses.
    standalone: sim.apple ? profile.standalone : undefined,
  }
}

// Answers the queries `@/helpers/platform` asks, and defers everything else to the real engine.
//
// Deferring matters: `matchMedia` is also used for `prefers-color-scheme`, `prefers-reduced-
// motion` and orientation elsewhere in the app, and a stub that answered `false` to all of them
// would quietly change unrelated behaviour while claiming only to simulate a phone.
function simulatedMatchMedia(profile: DevDevice): (query: string) => { matches: boolean } {
  return (query: string) => {
    if (query.includes('display-mode: standalone')) return { matches: profile.standalone }
    if (query.includes('display-mode: browser')) return { matches: !profile.standalone }
    if (query.includes('pointer: coarse')) return { matches: profile.mobile }
    if (typeof globalThis.matchMedia === 'function') return globalThis.matchMedia(query)
    return { matches: false }
  }
}

/** The environment a given profile should present to the detection helpers. */
export function simulatedEnv(profile: DevDevice): PlatformEnv {
  return {
    navigator: simulatedNavigator(profile),
    matchMedia: simulatedMatchMedia(profile),
  }
}
