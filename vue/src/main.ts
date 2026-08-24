import { createApp } from 'vue'
import { createPinia } from 'pinia'

import '@/assets/main.css'

import App from '@/App.vue'
import router from '@/router'
import { initPwa } from '@/helpers/pwa'
import { useAppStore } from '@/stores/app.store'

const app = createApp(App)

app.use(createPinia())
app.use(router)

app.mount('#app')

// eslint-disable-next-line no-console
console.info(`Hej Nathejk v${__APP_VERSION__}`)

// Register the service worker (installability + update detection). When a new
// build is waiting, flag it on the app store; UpdatePrompt (task 020) shows the
// reload affordance.
initPwa(() => {
  useAppStore().setUpdateAvailable(true)
})
