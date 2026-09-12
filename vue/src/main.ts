import { createApp } from 'vue'
import { createPinia } from 'pinia'

import '@/assets/main.css'

import App from '@/App.vue'
import router from '@/router'
import { initPwa } from '@/helpers/pwa'
import { useAppStore } from '@/stores/app.store'
import { initInstallPrompt } from '@/stores/install.store'
import { loadRuntimeConfig } from '@/config/runtime'
import { initGateOverride } from '@/config/gates'
import { initSafeArea } from '@/helpers/safeArea'

const app = createApp(App)

app.use(createPinia())
app.use(router)

// Before mount: components read var(--sat)/--sab for their padding, and main.css seeds
// those from env(). This corrects the seed for the one case CSS cannot detect (see
// @/helpers/safeArea), and doing it first avoids a visible reflow of the shell.
initSafeArea()

// Before the router's first navigation, so `?nogate=1` is already persisted when the very
// first gate check runs (task 139). Inert in production builds.
initGateOverride()

// Also before the first navigation, and for the same reason: the gates read the simulated
// device on their very first check, so a profile applied later would show up as a redirect
// flash on every start.
//
// Note what this deliberately is *not*: `initGateOverride` above switches the gates off, while
// this leaves them on and changes what they *see*. That distinction is the whole reason PRD 014
// exists — the bypass also disables the onboarding redirect, so it cannot be used to test
// onboarding.
//
// A dynamic import inside the `DEV` branch, not a static one at the top of the file: Rollup
// folds the condition away in a production build and drops the branch, so the dev layer emits
// no chunk and contributes no strings to `dist/`. A static import with a guarded call site does
// not achieve that — it was measured putting the simulation's fake user-agent strings into the
// production bundle. Top-level await is fine here: the baseline is Safari 16.4+ / Chrome 111+,
// and in production this code does not exist at all.
if (import.meta.env.DEV) {
  const { initDevSimulation } = await import('@/dev/bootstrap')
  initDevSimulation()
}

// Before mount, and this position is load-bearing: `beforeinstallprompt` fires once and
// early, so a listener registered after the app has mounted misses it outright. When that
// happens nothing errors — the install wall just silently shows the manual
// add-to-home-screen instructions instead of the one-tap button, on Chromium, the only
// platform where one tap exists (PRD 005 §8). Do not move this below `app.mount()`.
initInstallPrompt()

app.mount('#app')

// Fetch the public runtime config once, at startup.
//
// Previously this was kicked off only by MapsView, which was enough while the map's
// Dataforsyningen token was the only value in it — but the config now also carries the
// diagnostic display flags, which are consumed by the app shell. That made them
// dependent on having visited the map first: the build id was missing from the nav on
// every other page until the map had been opened once, which is precisely the kind of
// "present in some screenshots, absent in others" behaviour it exists to eliminate.
//
// Not awaited: nothing here should delay the first paint. Consumers are refs that flip
// when it resolves, and MapsView still awaits the same shared in-flight promise, so
// calling it early only warms it.
void loadRuntimeConfig()

// eslint-disable-next-line no-console
console.info(`Hej Nathejk v${__APP_VERSION__}`)

// Register the service worker (installability + update detection). When a new
// build is waiting, flag it on the app store; UpdatePrompt (task 020) shows the
// reload affordance.
initPwa(() => {
  useAppStore().setUpdateAvailable(true)
})
