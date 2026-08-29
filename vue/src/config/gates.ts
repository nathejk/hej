import { installGateEnabled } from '@/config/runtime'

// The seam the install/device gates are switched off at (PRD 005 §6, §10).
//
// One thing for the router guard to ask, so the guard does not grow knowledge of runtime
// config, query parameters and QA flags.

// Query parameter and localStorage key for the dev/QA bypass.
//
// **Both forms are needed**, and the reason is structural rather than convenience: the
// manifest's `start_url` is `/`, so an installed home-screen launch drops the query string
// entirely — i.e. the query form is unavailable in exactly the mode the gate exists to
// produce. The same constraint is already recorded in `runtime.ts` for `?debug=`.
//
// So `?nogate=1` sets the localStorage flag and the flag is what is actually consulted;
// `?nogate=0` clears it again, because a QA override with no off switch is one that gets
// left on.
const OVERRIDE_PARAM = 'nogate'
const OVERRIDE_KEY = 'hej.gates.bypass'

// Vite replaces this at build time, so a production bundle cannot be talked into the
// bypass by a URL: the branch is compiled out.
const isProd = import.meta.env.PROD

function readOverride(): boolean {
  try {
    return localStorage.getItem(OVERRIDE_KEY) === '1'
  } catch {
    return false
  }
}

function writeOverride(on: boolean) {
  try {
    if (on) localStorage.setItem(OVERRIDE_KEY, '1')
    else localStorage.removeItem(OVERRIDE_KEY)
  } catch {
    // Blocked storage costs the override its persistence, which is a dev-only nuisance.
  }
}

/**
 * Reads `?nogate=1` / `?nogate=0` and persists it. Called once from `main.ts`, before the
 * router's first navigation, so the very first gate check already sees it.
 *
 * Inert in production builds.
 */
export function initGateOverride(search: string = window.location.search) {
  if (isProd) return
  const value = new URLSearchParams(search).get(OVERRIDE_PARAM)
  if (value === null) return
  writeOverride(value !== '0' && value !== 'false')
}

/**
 * Whether the device/install/onboarding gates should run at all.
 *
 * Two independent reasons they might not, and they are deliberately different mechanisms:
 *
 * - **The runtime flag** (`install_gate` from `GET /api/config`) is an operational kill
 *   switch. If detection misfires during an event, participants cannot reach the map, the
 *   SOS page or their contacts, and a redeploy is not an acceptable response time.
 * - **The dev/QA override** exists so the rest of the flow can be exercised in a normal
 *   browser tab. It is **not** the answer to "an organizer needs laptop access" — PRD 005
 *   §11 says that would be a new PRD, not a flag.
 *
 * Deliberately synchronous: the guard consults it on every navigation, including the first
 * paint on a cold start, and an awaited answer would produce the redirect flash PRD 005 §6
 * rules out. `installGateEnabled` is a ref that starts `true` and is corrected when the
 * config resolves, so an early navigation gates by default rather than by accident.
 */
export function gatesEnabled(): boolean {
  if (!installGateEnabled.value) return false
  if (!isProd && readOverride()) return false
  return true
}
