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
