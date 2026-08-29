import { readonly, ref } from 'vue'

// Public configuration fetched from the BFF at startup rather than inlined by
// Vite at build time, so one published image can run in any environment. See
// GET /api/config (go/cmd/api/config.go).
//
// Only ever put values here that are safe to hand to any browser — this is a
// public endpoint, and the values end up in the running page either way.

interface RuntimeConfigResponse {
  dataforsyningen_token?: string
  show_build_id?: boolean
  show_layout_debug?: boolean
  install_gate?: boolean
}

const token = ref('')
const showBuild = ref(false)
const showLayout = ref(false)
// PRD 005's install gate. Defaults ON before the config has loaded, and stays on if the
// request fails with nothing remembered: a gate that fails open would let a participant
// straight past onboarding on a cold, offline first start — which is the one path where
// nothing else would ever ask them for location or notifications.
const installGate = ref(true)

// The last token the BFF handed us, remembered so the map still has a key offline
// (task 090).
//
// Safe to persist: this is a public quota key already served to any browser that
// asks, and it is the *same* value the next online start would fetch. Without it, an
// offline start had no token, so the map told the user their deployment was missing
// an API key — a configuration error they cannot act on and that isn't true. A
// network failure must never be reported as a misconfiguration.
const STORAGE_KEY = 'hej.dataforsyningen-token'
// The diagnostic flags are remembered for the same reason as the token: an offline
// start would otherwise switch them off, and offline is the normal case in the field —
// which is exactly when a screenshot needs to say which build produced it, and when a
// layout question is hardest to reproduce afterwards.
const SHOW_BUILD_KEY = 'hej.show-build-id'
const SHOW_LAYOUT_KEY = 'hej.show-layout-debug'
// Remembered for the same reason as the diagnostic flags, and with more at stake: an
// offline start must not silently flip the gate's behaviour. If an organizer has switched
// the gate off mid-event, a member whose phone starts without coverage must still get the
// ungated app rather than the wall.
const INSTALL_GATE_KEY = 'hej.install-gate'

function remembered(): string {
  try {
    return localStorage.getItem(STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

function remember(value: string) {
  try {
    if (value) localStorage.setItem(STORAGE_KEY, value)
    else localStorage.removeItem(STORAGE_KEY)
  } catch {
    // Blocked or full storage only costs the offline case.
  }
}

function rememberFlag(key: string, value: boolean) {
  try {
    localStorage.setItem(key, value ? '1' : '0')
  } catch {
    // Blocked or full storage only costs the offline case.
  }
}

function rememberedFlag(key: string): boolean {
  try {
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}

// Like rememberedFlag, but for a flag whose default is not `false`. The install gate needs
// this: "never heard from the server" and "the server said off" must not collapse into the
// same answer for a switch that decides whether the app is reachable at all.
function rememberedFlagOr(key: string, fallback: boolean): boolean {
  try {
    const raw = localStorage.getItem(key)
    if (raw === null) return fallback
    return raw === '1'
  } catch {
    return fallback
  }
}

/** Dataforsyningen quota key for the map's WMS base layers; '' until loaded. */
export const dataforsyningenToken = readonly(token)

/**
 * Whether to overlay the build id on the bottom nav. Diagnostic only — the privacy
 * page shows the build id regardless of this flag.
 */
export const showBuildId = readonly(showBuild)

/**
 * Whether to overlay viewport / safe-area / geometry values on the app. Diagnostic
 * only, off unless SHOW_LAYOUT_DEBUG is set on the BFF.
 *
 * Deliberately not driven by a `?debug=` URL parameter: the manifest's start_url is
 * "/", so an installed home-screen launch drops the query string — i.e. it would be
 * unavailable in standalone mode, the only mode where these values differ from a
 * browser tab and the only one worth debugging.
 */
export const showLayoutDebug = readonly(showLayout)

/**
 * Whether PRD 005's install gate is active: device classification, the install wall and
 * the onboarding redirect.
 *
 * A kill switch rather than a feature toggle — see `config/gates.ts`, which is what the
 * router guard consults.
 */
export const installGateEnabled = readonly(installGate)

// Module-level so concurrent callers share one request and later callers resolve
// immediately — the map and any future consumer can each `await` it freely.
let inFlight: Promise<void> | null = null

export function loadRuntimeConfig(): Promise<void> {
  if (inFlight) {
    return inFlight
  }
  inFlight = (async () => {
    try {
      const res = await fetch('/api/config', { credentials: 'same-origin' })
      if (!res.ok) {
        throw new Error(`GET /api/config: ${res.status}`)
      }
      const body = (await res.json()) as RuntimeConfigResponse
      token.value = body.dataforsyningen_token ?? ''
      showBuild.value = body.show_build_id ?? false
      showLayout.value = body.show_layout_debug ?? false
      // Absent means on, not off. An older BFF that does not send the field must not
      // disable the gate — the safe direction for a missing value here is the gate's
      // designed behaviour.
      installGate.value = body.install_gate ?? true
      // Deliberately mirrors an unset key too, so clearing it in production
      // eventually clears it on the device rather than living on forever.
      remember(token.value)
      rememberFlag(SHOW_BUILD_KEY, showBuild.value)
      rememberFlag(SHOW_LAYOUT_KEY, showLayout.value)
      rememberFlag(INSTALL_GATE_KEY, installGate.value)
    } catch (err) {
      // Degrade rather than block the page: fall back to the last known token so an
      // offline start still draws map tiles. If there is no remembered token either,
      // an empty value makes the map show its "missing key" notice — the same
      // outcome as an unconfigured deployment, and far better than a view that never
      // renders.
      token.value = remembered()
      showBuild.value = rememberedFlag(SHOW_BUILD_KEY)
      showLayout.value = rememberedFlag(SHOW_LAYOUT_KEY)
      // Defaults to ON when nothing is remembered, unlike the diagnostics: this is the
      // app's designed behaviour, and a first offline start must not skip onboarding.
      installGate.value = rememberedFlagOr(INSTALL_GATE_KEY, true)
      console.error('failed to load runtime config', err)
      // Allow a later retry (e.g. revisiting the map after a network blip).
      inFlight = null
    }
  })()
  return inFlight
}
