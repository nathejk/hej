import { readDevDevice, type DevDevice, type DevPlatform } from '@/config/devDevice'

// Device and install-state detection — the single place the app decides what kind of
// device it is running on and whether it is running installed (PRD 005 §8).
//
// The router gate, the install wall, the install instructions and the desktop
// placeholder all consume this. Three constraints shape the file:
//
// 1. **Synchronous and dependency-free.** The router guard has to decide during
//    `beforeEach` on a cold start, or the user sees a redirect flash. That rules out
//    anything awaited (`navigator.permissions.query`, `getInstalledRelatedApps`,
//    dynamic imports), and it rules out importing stores or config — the guard must not
//    pick up a Pinia dependency it does not need.
// 2. **Injectable environment.** Nothing here reads a global at module level and nothing
//    is cached at import time. The awkward cases this file exists for — iPadOS claiming
//    to be a Mac, an in-app webview — cannot be reproduced on the machine running the
//    tests, so they have to be handed in.
// 3. **No viewport width, anywhere** (PRD 005 §6). Width is not a device class: a phone
//    in landscape, a split-screen tablet and a narrow desktop window are
//    indistinguishable by it, and what this feeds decides whether a participant can
//    reach the app at all.
//
// Baseline is iOS/iPadOS Safari 16.4+ / Chrome 111+, so absent APIs are guarded
// (`userAgentData` is Chromium-only, `navigator.standalone` is WebKit-only) but nothing
// is polyfilled.
//
// # The dev device profile (PRD 014, task 207)
//
// `defaultEnv()` yields a **synthesised** environment when a dev profile is active, so the
// app can be walked on a laptop. Two things about how that is done are deliberate:
//
// - **It enters through the environment, not through the predicates.** Everything below the
//   default argument is untouched, so `platform.spec.ts` stays valid as written and the
//   simulation runs through the real heuristics rather than around them — a simulated iPad
//   is a `MacIntel` navigator with touch points, and it classifies as mobile because
//   `isAppleTouchDevice` says so, not because something short-circuited.
// - **It does not violate constraint 2 above.** `@/config/devDevice` imports nothing, holds
//   no state and does one synchronous `localStorage` read; the constraint is against awaited
//   work and Pinia, both of which the guard genuinely cannot afford. The read is uncached
//   here for the same reason it is uncached there: the dev panel changes it at runtime.
//
// All of it is compiled out of a production build (`import.meta.env.PROD` in devDevice).

/** The narrow slice of `navigator` this module reads. Structural on purpose, so a test
 *  can supply three fields instead of a whole `Navigator`. */
export interface PlatformNavigator {
  userAgent: string
  /** Legacy, but still the only way to spot iPadOS: it reports `'MacIntel'`. */
  platform?: string
  maxTouchPoints?: number
  /** Chromium-only (`navigator.userAgentData`). */
  userAgentData?: { mobile?: boolean; brands?: { brand: string }[] }
  /** WebKit-only, and true only in a home-screen web app. */
  standalone?: boolean
}

export interface PlatformEnv {
  navigator: PlatformNavigator
  matchMedia: (query: string) => { matches: boolean }
}

/** Where the app is running, for the purpose of telling the user how to install it. */
export type InstallPlatform = 'chromium' | 'ios-safari' | 'other' | 'webview'

// Display modes that all mean "launched as an installed app". `minimal-ui` and
// `fullscreen` are as installed as `standalone` — a manifest may legitimately ask for
// either, and treating them as "in a browser tab" would send an installed user back to
// the install wall forever.
// Display modes, and which of them can mean "launched from the home screen".
//
// **Only `standalone`**, because that is the only mode this app's manifest asks for
// (`display: 'standalone'` in vite.config.ts). It is tempting to accept `minimal-ui` and
// `fullscreen` as "equally installed" — an earlier version of this file did — but since the
// manifest never requests them, they cannot appear on an installed launch. They can only
// appear on an *uninstalled* one, which makes them pure false positives:
//
//   - `fullscreen` matches whenever the browsing context is fullscreen. Play a video
//     fullscreen in an ordinary tab and the tab starts claiming to be an installed app.
//   - `minimal-ui` is matched by several mobile browsers for their own chrome-reduced
//     reading modes (Samsung Internet, Firefox for Android), in a plain tab.
//
// Getting this wrong is not cosmetic: a tab that reads as installed skips the install wall
// and drops the user straight into onboarding in a browser, which is precisely the
// configuration PRD 005 exists to prevent — no Web Push on iOS, no reliable service worker.
const INSTALLED_DISPLAY_MODE = 'standalone'

// The mode a real browser tab reports. Checked as a *veto* below.
const BROWSER_DISPLAY_MODE = 'browser'

// In-app browsers. Facebook and Instagram are the ones that matter here: participants
// arrive from a link in a Facebook group, and installation is simply impossible in
// those webviews — no beforeinstallprompt, no Share → Add to Home Screen.
const WEBVIEW_MARKERS = [
  'FBAN',
  'FBAV',
  'FB_IAB',
  'Instagram',
  'Snapchat',
  'Line/',
  'MicroMessenger',
  'GSA/', // Google App's in-app browser
]

function defaultEnv(): PlatformEnv {
  const simulated = devEnv()
  if (simulated) return simulated
  return {
    navigator: globalThis.navigator as unknown as PlatformNavigator,
    matchMedia: (query: string) => globalThis.matchMedia(query),
  }
}

function isAppleTouchDevice(nav: PlatformNavigator): boolean {
  if (/iPhone|iPod|iPad/.test(nav.userAgent)) return true
  // iPadOS 13+ requests desktop sites by default: the UA says Macintosh and
  // `platform` says MacIntel. A Mac has no touch points, so this pair is the only
  // reliable tell. It is also why an iPad cannot be recognised from the UA alone.
  return nav.platform === 'MacIntel' && (nav.maxTouchPoints ?? 0) > 1
}

/**
 * Phone or tablet (a device where installing a PWA makes sense) vs. a desktop computer.
 *
 * The tie-break is deliberate: **ambiguous signals resolve to mobile** (PRD 005 §11,
 * 2026-08-30). Detection cannot be made exact — iPadOS reports itself as macOS Safari,
 * and touchscreen laptops answer yes to every touch question — so the question is only
 * which way to be wrong. The harms are not symmetric:
 *
 * - Desktop misread as mobile: the user taps "Fortsæt i browseren" once.
 * - iPad misread as desktop: the user is left on a placeholder page with no route into
 *   the app at all, during an event, for a safety app.
 *
 * So the negative branch is the narrow one: we return `false` only for a device that
 * shows no touch capability whatsoever. Do not tighten this without reading that
 * decision — the escape hatch and this tie-break are a pair, and removing either
 * breaks the other.
 */
export function isMobileDevice(env: PlatformEnv = defaultEnv()): boolean {
  const nav = env.navigator

  // Only the positive answer is decisive. Chrome on an Android *tablet* reports
  // `mobile: false`, so treating false as "desktop" would exclude exactly the tablets
  // PRD 005 targets.
  if (nav.userAgentData?.mobile === true) return true
  if (isAppleTouchDevice(nav)) return true

  if ((nav.maxTouchPoints ?? 0) > 0) return true
  if (env.matchMedia('(pointer: coarse)').matches) return true
  if (env.matchMedia('(any-pointer: coarse)').matches) return true

  // Mouse-only, no touch, not an Apple touch device: a desktop computer.
  return false
}

/**
 * Is this launch running installed, rather than in a browser tab?
 *
 * Three checks, in this order, and the order is the design:
 *
 * 1. **iOS's `navigator.standalone`** — WebKit-only, and true only in a home-screen web app.
 *    First because it is the one unambiguous signal we get on the platform where installing
 *    matters most (iOS gives Web Push to home-screen apps only).
 * 2. **An explicit `display-mode: browser` veto.** Only a real tab reports `browser`, so if
 *    the engine says so, nothing else should be able to override it. This is what keeps a
 *    fullscreen video or a chrome-less reading mode from being mistaken for an installed app.
 * 3. **`display-mode: standalone`** — the mode this app's manifest actually requests.
 *
 * Erring towards "not installed" is the safe direction: the cost is showing the install wall
 * to someone who is already installed (who then taps "jeg har allerede installeret appen"),
 * while the cost of the opposite is a participant using the app in a tab all event with no
 * notifications — silently, because nothing in the UI would say so.
 */
export function isStandalone(env: PlatformEnv = defaultEnv()): boolean {
  if (env.navigator.standalone === true) return true
  if (env.matchMedia(`(display-mode: ${BROWSER_DISPLAY_MODE})`).matches) return false
  return env.matchMedia(`(display-mode: ${INSTALLED_DISPLAY_MODE})`).matches
}

/** Which set of install instructions applies (task 120). */
export function installPlatform(env: PlatformEnv = defaultEnv()): InstallPlatform {
  const nav = env.navigator
  const ua = nav.userAgent

  // Checked first, and deliberately: an in-app webview is often Chromium underneath and
  // would otherwise be told to tap an install button it will never be offered.
  if (WEBVIEW_MARKERS.some((marker) => ua.includes(marker))) return 'webview'
  // Android WebView proper. The `; wv)` token is what distinguishes it from Chrome.
  if (/;\s?wv\)/.test(ua)) return 'webview'

  if (isAppleTouchDevice(nav)) {
    // Every browser on iOS is WebKit, and only Safari can add to the home screen —
    // Chrome/Firefox on iOS have no such affordance. So the iOS instructions are the
    // Safari ones regardless of which browser is showing them; the wall's copy has to
    // tell a Chrome-on-iOS user to switch, which it can only do if we land here.
    return 'ios-safari'
  }

  if (nav.userAgentData?.brands?.some((b) => /Chromium|Google Chrome|Microsoft Edge/.test(b.brand)))
    return 'chromium'
  if (/Chrome|Chromium|Edg\//.test(ua)) return 'chromium'

  // Android Firefox, Samsung Internet on an old build, anything else: it may or may not
  // support installation, so it gets the generic manual instructions.
  return 'other'
}

// ---------------------------------------------------------------------------
// Dev device simulation (PRD 014). Everything below is dev-only and compiled out of a
// production build, because `readDevDevice` is inert under `import.meta.env.PROD`.
// ---------------------------------------------------------------------------

// One plausible navigator per simulated platform.
//
// These are **real user-agent strings**, not tokens invented to satisfy the checks above, and
// that is the point: the simulation is fed through the same heuristics a real device is, so
// `?dev=ipad` exercises the `MacIntel` + touch-points path rather than bypassing it. If a
// heuristic is wrong, simulating it should reproduce the wrongness — a simulation that
// answers correctly by construction would hide exactly the bug the matrix in task 139 exists
// to find.
const SIMULATED: Record<DevPlatform, Omit<PlatformNavigator, 'standalone'>> = {
  ios: {
    userAgent:
      'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1',
    platform: 'iPhone',
    maxTouchPoints: 5,
  },
  // The awkward one, and the reason it is worth simulating at all: iPadOS 13+ requests
  // desktop sites, so the UA says Macintosh and only the touch points give it away.
  ipad: {
    userAgent:
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15',
    platform: 'MacIntel',
    maxTouchPoints: 5,
  },
  android: {
    userAgent:
      'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36',
    maxTouchPoints: 5,
    userAgentData: { mobile: true, brands: [{ brand: 'Google Chrome' }, { brand: 'Chromium' }] },
  },
  // Deliberately the same device as `android`. `chromium` is what `installPlatform()` returns,
  // so it is the word a developer reaches for when they want to see that branch; making it an
  // alias is cheaper than making them remember which vocabulary applies where.
  chromium: {
    userAgent:
      'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36',
    maxTouchPoints: 5,
    userAgentData: { mobile: true, brands: [{ brand: 'Google Chrome' }, { brand: 'Chromium' }] },
  },
  // Facebook's in-app browser on iOS — the case participants arriving from a Facebook group
  // actually hit, where installation is impossible and the wall has to say so.
  webview: {
    userAgent:
      'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/468.0.0.32.107]',
    platform: 'iPhone',
    maxTouchPoints: 5,
  },
  // A mobile browser that is neither WebKit-on-iOS nor Chromium: Firefox. No `Android` token,
  // so it also reaches the generic branch of `detectPlatform()` in @/config/permissions — the
  // one whose copy is otherwise unreachable from any simulated device.
  other: {
    userAgent: 'Mozilla/5.0 (Mobile; rv:127.0) Gecko/127.0 Firefox/127.0',
    maxTouchPoints: 5,
  },
}

// A desktop computer, for the (write-only) case of a profile that says `mobile: false`. No
// preset produces one — `?dev=desktop` clears the profile instead, which is better because it
// restores *real* detection — but the profile is a stored value and this file does not get to
// assume which shapes reach it.
const SIMULATED_DESKTOP: Omit<PlatformNavigator, 'standalone'> = {
  userAgent:
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36',
  platform: 'MacIntel',
  maxTouchPoints: 0,
  userAgentData: { mobile: false, brands: [{ brand: 'Google Chrome' }] },
}

function simulatedNavigator(profile: DevDevice): PlatformNavigator {
  const base = profile.mobile ? SIMULATED[profile.platform] : SIMULATED_DESKTOP
  const apple = isAppleTouchDevice(base as PlatformNavigator)
  return {
    ...base,
    // WebKit-only, so it is set only for the Apple devices. Elsewhere the display-mode query
    // below is what answers, which is also how a real Android device behaves — setting both
    // would make the simulation pass `isStandalone()` for a reason no real device uses.
    standalone: apple ? profile.standalone : undefined,
  }
}

/**
 * The simulated `PlatformNavigator` for the active dev profile, or `null` when there is none.
 *
 * Exported for `@/config/permissions`, so `detectPlatform()` can sniff the simulated UA
 * instead of growing its own copy of the platform mapping. That keeps its claim to be the
 * app's only user-agent sniff true, and keeps one source of truth for what an iPad looks like.
 */
export function devNavigator(): PlatformNavigator | null {
  const profile = readDevDevice()
  return profile ? simulatedNavigator(profile) : null
}

// Answers the queries this module asks, and defers everything else to the real engine.
//
// Deferring matters: `matchMedia` is also used for `prefers-color-scheme`, `prefers-reduced-
// motion` and orientation elsewhere in the app, and a stub that answered `false` to all of
// them would quietly change unrelated behaviour while claiming to simulate a phone.
function simulatedMatchMedia(profile: DevDevice): (query: string) => { matches: boolean } {
  return (query: string) => {
    if (query.includes(`display-mode: ${INSTALLED_DISPLAY_MODE}`)) {
      return { matches: profile.standalone }
    }
    if (query.includes(`display-mode: ${BROWSER_DISPLAY_MODE}`)) {
      return { matches: !profile.standalone }
    }
    if (query.includes('pointer: coarse')) return { matches: profile.mobile }
    if (typeof globalThis.matchMedia === 'function') return globalThis.matchMedia(query)
    return { matches: false }
  }
}

function devEnv(): PlatformEnv | null {
  const profile = readDevDevice()
  if (!profile) return null
  return {
    navigator: simulatedNavigator(profile),
    matchMedia: simulatedMatchMedia(profile),
  }
}
