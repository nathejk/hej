import { createApp } from 'vue'
import { createPinia } from 'pinia'

import '@/assets/main.css'

import App from '@/App.vue'
import router from '@/router'
import { initPwa } from '@/helpers/pwa'
import { useAppStore } from '@/stores/app.store'
import { loadRuntimeConfig } from '@/config/runtime'
import { initSafeArea } from '@/helpers/safeArea'

const app = createApp(App)

app.use(createPinia())
app.use(router)

// Before mount: components read var(--sat)/--sab for their padding, and main.css seeds
// those from env(). This corrects the seed for the one case CSS cannot detect (see
// @/helpers/safeArea), and doing it first avoids a visible reflow of the shell.
initSafeArea()

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
