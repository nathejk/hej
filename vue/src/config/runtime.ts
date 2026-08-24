import { readonly, ref } from 'vue'

// Public configuration fetched from the BFF at startup rather than inlined by
// Vite at build time, so one published image can run in any environment. See
// GET /api/config (go/cmd/api/config.go).
//
// Only ever put values here that are safe to hand to any browser — this is a
// public endpoint, and the values end up in the running page either way.

interface RuntimeConfigResponse {
  dataforsyningen_token?: string
}

const token = ref('')

/** Dataforsyningen quota key for the map's WMS base layers; '' until loaded. */
export const dataforsyningenToken = readonly(token)

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
    } catch (err) {
      // Degrade rather than block the page: an empty token makes the map show
      // its "missing key" notice, which is the same outcome as an unconfigured
      // deployment and far better than a view that never renders.
      console.error('failed to load runtime config', err)
      // Allow a later retry (e.g. revisiting the map after a network blip).
      inFlight = null
    }
  })()
  return inFlight
}
