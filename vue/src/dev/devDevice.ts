// The dev device profile: what device the app should *believe* it is running on (PRD 014).
//
// # Why this is not another gate bypass
//
// One already exists — `?nogate=1` → `hej.gates.bypass` → `gatesEnabled()` in
// `@/config/gates` — and it is the wrong tool for development, for a structural reason
// rather than a stylistic one: it disables guard steps 2, 3 **and 4** together (see
// `router/index.ts`, which consults it once). Step 4 is the onboarding redirect, so the only
// current route onto a laptop is also the one that skips the flow most in need of testing.
// `gates.ts` says so itself and declines to solve it ("that would be a new PRD").
//
// So this module does the opposite of a bypass: it makes the app *lie about the hardware*
// and leaves every gate switched on. `@/helpers/platform` and `@/config/permissions` read
// this profile (task 207), the guard then runs steps 2–4 for real, and `?dev=iphone` lands
// on `/welcome` through genuine onboarding gating rather than around it.
//
// `config/gates.ts` is deliberately untouched: `?nogate=` keeps its own meaning as the way
// to verify the `install_gate` kill switch.
//
// # Injectable environment, for the same reason as platform.ts
//
// Nothing here reads a global at module level and nothing is cached at import time. Two
// reasons, and the second is the one that bites:
//
// 1. Vitest runs `environment: 'node'` (see vitest.config.ts), so there is no
//    `localStorage` to read and no production build to be inside. Both have to be handed in
//    or neither can be tested — and the `PROD` inertness is this module's single most
//    important property.
// 2. The profile changes at runtime, from the dev panel (task 208). A value memoised at
//    import time would mean every toggle needed a reload, and worse, would make the
//    *readout* disagree with the behaviour — which is precisely the confusion the panel
//    exists to prevent.

/** Which platform to impersonate. Maps onto both `installPlatform()` and `detectPlatform()`
 *  in task 207; neither vocabulary is used here, so this file owns neither. */
export type DevPlatform = 'ios' | 'ipad' | 'android' | 'chromium' | 'webview' | 'other'

export interface DevDevice {
  mobile: boolean
  standalone: boolean
  platform: DevPlatform
}

// The `hej.dev.` prefix is load-bearing, not cosmetic. This origin already holds product
// keys — `hej.gates.bypass`, `hej.install-gate`, `hej.show-build-id`, `hej.show-layout-debug`,
// `hej.dataforsyningen-token`, `hej.contacts-poll-seconds`, `hej.onboarding.*` — and a dev
// override that is indistinguishable from them is one that gets cleared by accident, or
// worse, not cleared when "clear all dev overrides" is asked for. The prefix makes that a
// scan rather than a hand-maintained list (see `clearDevOverrides`).
export const DEV_KEY_PREFIX = 'hej.dev.'

/** Where the profile lives. */
export const DEV_DEVICE_KEY = `${DEV_KEY_PREFIX}device`

/** The query parameter, e.g. `?dev=iphone`. */
export const DEV_PARAM = 'dev'

const PLATFORMS: readonly DevPlatform[] = ['ios', 'ipad', 'android', 'chromium', 'webview', 'other']

/**
 * The presets `?dev=` accepts. `null` means "clear the profile".
 *
 * Notes on the three that are not simply a platform name:
 *
 * - **`tab`** is mobile but *not* standalone — a phone in a browser tab. It is how the
 *   install wall gets tested, which is otherwise unreachable on a laptop: a laptop with no
 *   profile never even sees the wall, because step 2 of the guard sends it out of the SPA
 *   to `/desktop.html` first. `tab` deliberately keeps whatever platform is already
 *   simulated (see `applyPreset`) so `?dev=android` then `?dev=tab` shows the *Chromium*
 *   wall rather than silently switching to iOS.
 * - **`webview`** cannot be standalone. An in-app browser has no add-to-home-screen at all,
 *   so a standalone webview is not a state that exists on any real device, and simulating
 *   one would test a branch no user can reach.
 * - **`desktop`** and **`off`** both clear. Two names for one action because they answer two
 *   different questions ("show me the desktop hand-off" and "stop simulating"), and a dev
 *   override with no off switch is one that gets left on — the same argument `gates.ts`
 *   already makes for `?nogate=0`.
 */
export const DEV_PRESETS: Readonly<Record<string, DevDevice | null>> = {
  iphone: { mobile: true, standalone: true, platform: 'ios' },
  ipad: { mobile: true, standalone: true, platform: 'ipad' },
  android: { mobile: true, standalone: true, platform: 'android' },
  chromium: { mobile: true, standalone: true, platform: 'chromium' },
  webview: { mobile: true, standalone: false, platform: 'webview' },
  // `platform` here is only the fallback; an existing profile's platform wins.
  tab: { mobile: true, standalone: false, platform: 'ios' },
  desktop: null,
  off: null,
}

/** The slice of `Storage` this module uses. Structural so a test can pass a Map-backed
 *  stub instead of standing up jsdom. `length`/`key` are optional because only the
 *  prefix scan needs them, and a stub that cannot enumerate is still useful. */
export interface DevStorage {
  getItem: (key: string) => string | null
  setItem: (key: string, value: string) => void
  removeItem: (key: string) => void
  length?: number
  key?: (index: number) => string | null
}

export interface DevEnv {
  /** `null` when storage is unavailable — Safari throws on access in some privacy modes,
   *  which is why every localStorage access in this app is wrapped. */
  storage: DevStorage | null
  /** True in a production build. Every read and write is inert when set. */
  prod: boolean
}

function realStorage(): DevStorage | null {
  try {
    return globalThis.localStorage ?? null
  } catch {
    return null
  }
}

function defaultEnv(): DevEnv {
  return {
    storage: realStorage(),
    // Vite replaces this at build time, so the branches below are compiled out of a
    // production bundle: a production build cannot be talked into a simulated device by a
    // URL, because there is no code left to talk to. Same technique as `gates.ts`.
    prod: import.meta.env.PROD,
  }
}

// Validated rather than trusted: what comes back from storage is input, and this particular
// input decides whether the app believes it is on a phone. A malformed value must read as
// "no simulation" rather than as a partially-populated profile, or the app ends up in a
// state no real device can be in (`mobile: undefined`) and the resulting bug looks like one
// in the gates.
function parse(raw: string | null): DevDevice | null {
  if (!raw) return null
  try {
    const value = JSON.parse(raw) as Partial<DevDevice>
    const platform = value?.platform
    if (typeof value?.mobile !== 'boolean') return null
    if (typeof value?.standalone !== 'boolean') return null
    if (!platform || !PLATFORMS.includes(platform)) return null
    return { mobile: value.mobile, standalone: value.standalone, platform }
  } catch {
    return null
  }
}

/**
 * The active dev device profile, or `null` when the app should trust the real hardware.
 *
 * Always `null` in production. Uncached on purpose — see the header.
 */
export function readDevDevice(env: DevEnv = defaultEnv()): DevDevice | null {
  if (env.prod) return null
  if (!env.storage) return null
  try {
    return parse(env.storage.getItem(DEV_DEVICE_KEY))
  } catch {
    return null
  }
}

/**
 * Sets or clears the profile. `null` clears.
 *
 * Inert in production, so nothing can persist a profile into a production build even by
 * calling this directly.
 */
export function writeDevDevice(value: DevDevice | null, env: DevEnv = defaultEnv()) {
  if (env.prod) return
  if (!env.storage) return
  try {
    if (value) env.storage.setItem(DEV_DEVICE_KEY, JSON.stringify(value))
    else env.storage.removeItem(DEV_DEVICE_KEY)
  } catch {
    // Blocked or full storage costs the override its persistence, which is a dev-only
    // nuisance. Same posture as `gates.ts` and `runtime.ts`.
  }
}

/**
 * Applies a named preset, resolving `tab`'s "keep the current platform" behaviour.
 *
 * Returns the resulting profile (`null` when cleared), or `undefined` when the name is not a
 * preset — which the caller distinguishes from "cleared", since ignoring a typo and clearing
 * the profile are very different outcomes to the person who typed it.
 */
export function applyPreset(name: string, env: DevEnv = defaultEnv()): DevDevice | null | undefined {
  if (!(name in DEV_PRESETS)) return undefined
  const preset = DEV_PRESETS[name]
  if (preset === null) {
    writeDevDevice(null, env)
    return null
  }
  const current = readDevDevice(env)
  const next: DevDevice =
    name === 'tab' && current ? { ...preset, platform: current.platform } : { ...preset }
  writeDevDevice(next, env)
  return next
}

/**
 * Removes every `hej.dev.*` key — the profile and whatever later phases add (panel state,
 * fake position, forced offline).
 *
 * Needs enumeration, so it is a no-op against a stub that cannot enumerate. Collects the
 * keys before deleting any: removing during iteration reindexes `Storage`, which silently
 * skips entries.
 */
export function clearDevOverrides(env: DevEnv = defaultEnv()) {
  if (env.prod) return
  const storage = env.storage
  if (!storage || typeof storage.key !== 'function' || typeof storage.length !== 'number') return
  try {
    const doomed: string[] = []
    for (let i = 0; i < storage.length; i += 1) {
      const key = storage.key(i)
      if (key?.startsWith(DEV_KEY_PREFIX)) doomed.push(key)
    }
    doomed.forEach((key) => storage.removeItem(key))
  } catch {
    // As above: dev-only nuisance.
  }
}

/**
 * Reads `?dev=<preset>` and persists the result. Called once from `main.ts`, **before
 * `app.use(router)`** — installing the router triggers its first navigation, and the device gate
 * answers a desktop by replacing the location synchronously, so anything later loses the race and
 * the query string leaves with the URL.
 *
 * Both a query form and a stored form exist, and the query form alone would not do: the
 * manifest's `start_url` is `/`, so an installed launch drops the query string. That
 * constraint is already recorded in `runtime.ts` (for `?debug=`) and `gates.ts` (for
 * `?nogate=`).
 *
 * **The parameter must be given on an app URL** — `/?dev=iphone`. An earlier version of this
 * comment claimed `/desktop.html?dev=iphone` would work "because this module runs on the next
 * load"; that is wrong, and it sent someone in a circle. `desktop.html` is a plain static file
 * with no bundle, so nothing there parses the query and reloading it just loads it again. The app
 * root is the only URL that reaches this code.
 *
 * Returns the profile now in effect, or `null`. Inert in production.
 */
export function initDevDevice(
  search: string = typeof window === 'undefined' ? '' : window.location.search,
  env: DevEnv = defaultEnv(),
): DevDevice | null {
  if (env.prod) return null

  const requested = new URLSearchParams(search).get(DEV_PARAM)
  if (requested === null) return readDevDevice(env)

  const result = applyPreset(requested.toLowerCase(), env)
  if (result === undefined) {
    // Loud, because the alternative is a developer concluding the layer is broken when they
    // have simply misremembered a preset name.
    // eslint-disable-next-line no-console
    console.warn(
      `[dev] unknown ?dev=${requested}. Try one of: ${Object.keys(DEV_PRESETS).join(', ')}`,
    )
    return readDevDevice(env)
  }
  return result
}
