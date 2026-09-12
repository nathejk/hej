import { setDevEnvProvider } from '@/helpers/platform'

import { initDevDevice, readDevDevice } from '@/dev/devDevice'
import { simulatedEnv } from '@/dev/platformSim'
import { initForcedOffline } from '@/dev/forcedOffline'
import { initFakeSafeArea } from '@/dev/fakeSafeArea'

// The single entry point into PRD 014's dev layer.
//
// `main.ts` reaches this by a **dynamic import inside `if (import.meta.env.DEV)`**, which is
// what actually keeps the layer out of production: Rollup folds the condition to `false`,
// drops the branch, and with it the import — so no chunk is emitted and none of the dev
// layer's strings (fake user agents, preset names, panel copy) reach `dist/`. Guarding the
// *use* of a statically imported module is not equivalent, and was measured not to work: the
// UA table ended up in `dist/assets/index-*.js`.
//
// Consequence worth stating: nothing in `src/dev/` may ever be imported statically from
// product code. `@/helpers/platform` therefore takes a registered provider rather than
// importing the simulation, and `App.vue` reaches the panel through an async component.

/**
 * Applies `?dev=` and registers the simulated environment.
 *
 * Must run **before the router's first navigation**, or the first gate check reads the real
 * device and the corrected answer arrives as a redirect flash. `main.ts` awaits it before
 * `app.mount()` for that reason.
 */
export function initDevSimulation() {
  initDevDevice()
  setDevEnvProvider(() => {
    const profile = readDevDevice()
    return profile ? simulatedEnv(profile) : null
  })
  // Registers the predicate only; nothing is forced until the panel asks. Safe to do this early
  // because the flag starts false and is not persisted.
  initForcedOffline()
  // Registers the provider only; it answers `null` until a preset is chosen, so the real device
  // is measured as before.
  initFakeSafeArea()
}
