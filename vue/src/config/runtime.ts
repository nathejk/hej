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
}

const token = ref('')
const showBuild = ref(false)

// The last token the BFF handed us, remembered so the map still has a key offline
// (task 090).
//
// Safe to persist: this is a public quota key already served to any browser that
// asks, and it is the *same* value the next online start would fetch. Without it, an
// offline start had no token, so the map told the user their deployment was missing
// an API key — a configuration error they cannot act on and that isn't true. A
// network failure must never be reported as a misconfiguration.
const STORAGE_KEY = 'hej.dataforsyningen-token'
// Remembered for the same reason as the token: an offline start would otherwise hide
// the build id, and offline is the normal case in the field — which is exactly when a
// screenshot needs to say which build produced it.
const SHOW_BUILD_KEY = 'hej.show-build-id'

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

/** Dataforsyningen quota key for the map's WMS base layers; '' until loaded. */
export const dataforsyningenToken = readonly(token)

/**
 * Whether to overlay the build id on the bottom nav. Diagnostic only — the privacy
 * page shows the build id regardless of this flag.
 */
export const showBuildId = readonly(showBuild)

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
      // Deliberately mirrors an unset key too, so clearing it in production
      // eventually clears it on the device rather than living on forever.
      remember(token.value)
      try {
        localStorage.setItem(SHOW_BUILD_KEY, showBuild.value ? '1' : '0')
      } catch {
        // Blocked or full storage only costs the offline case.
      }
    } catch (err) {
      // Degrade rather than block the page: fall back to the last known token so an
      // offline start still draws map tiles. If there is no remembered token either,
      // an empty value makes the map show its "missing key" notice — the same
      // outcome as an unconfigured deployment, and far better than a view that never
      // renders.
      token.value = remembered()
      try {
        showBuild.value = localStorage.getItem(SHOW_BUILD_KEY) === '1'
      } catch {
        showBuild.value = false
      }
      console.error('failed to load runtime config', err)
      // Allow a later retry (e.g. revisiting the map after a network blip).
      inFlight = null
    }
  })()
  return inFlight
}
