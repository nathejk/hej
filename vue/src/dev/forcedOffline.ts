import { ref } from 'vue'

import { setDevNetworkBlocker } from '@/helpers/fetchWrapper'
import { useAppStore } from '@/stores/app.store'

// The force-offline toggle (PRD 014, task 210).
//
// # Why not DevTools
//
// Chrome's "Offline" throttling kills the Vite dev server's HMR websocket, so the page stops
// updating and the next edit appears not to work — every offline test then costs a dev-server
// restart. Network throttling also cannot express the state that actually matters in the field.
//
// # Why this is a supported input rather than a poke into private state
//
// `app.store`'s comment on `online` is the design: `navigator.onLine` only means "this device
// has a network interface with a route" — true on a captive portal, true with one unusable bar,
// true on the event's own patchy coverage. So the flag is *seeded* from `onLine` and then
// corrected by what actually happens, with `fetchWrapper`'s `NetworkError` driving it via
// `session.store`. The store is already built to be told it is offline by something other than
// the browser; this is one more such thing.
//
// # Two levels
//
// 1. The flag alone produces every "you are offline" affordance: the shell indicator, the
//    offline notice, the readiness view's framing.
// 2. Requests actually failing is what catches the interesting bugs — a component that renders
//    an offline banner while still happily awaiting a fetch is only distinguishable under 2.
//
// Both are on together, because a flag that says offline while requests succeed is exactly the
// kind of half-truth PRD 014 forbids the layer from producing.
//
// # Not persisted, deliberately
//
// In memory only, so a reload clears it. A forgotten force-offline looks precisely like a broken
// BFF, and the developer would have no reason to suspect their own override — the same trap the
// device profile's always-visible marker exists to prevent, but here a reload is a cheaper
// mitigation than a badge.

const forced = ref(false)

/** Whether requests are currently being failed on purpose. */
export const forcedOffline = forced

export function setForcedOffline(on: boolean) {
  forced.value = on
  const app = useAppStore()
  if (on) {
    app.setOnline(false)
    return
  }
  // Restored from the browser rather than assumed true: the machine may genuinely be offline,
  // and claiming otherwise would be the same lie in the other direction.
  app.setOnline(typeof navigator === 'undefined' ? true : navigator.onLine)
}

/**
 * Wires the predicate into `fetchWrapper`. Called once from `@/dev/bootstrap`.
 *
 * There is deliberately **no** second connectivity flag here: `offline.store` documents that it
 * holds none, because `app.store` owns connectivity and a copy "would drift from it, and the two
 * would disagree in exactly the situation both exist for".
 */
export function initForcedOffline() {
  setDevNetworkBlocker(() => forced.value)
}
