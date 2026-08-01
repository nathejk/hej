import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import Lara from '@primevue/themes/lara'
import ToastService from 'primevue/toastservice'

import '@/assets/main.css'

import App from '@/App.vue'
import router from '@/router'
import { initPwa } from '@/helpers/pwa'
import { useAppStore } from '@/stores/app.store'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(PrimeVue, {
  theme: {
    preset: Lara,
    options: {
      // Keep PrimeVue's generated theme layer below Tailwind utilities so
      // utility classes win when they collide.
      cssLayer: {
        name: 'primevue',
        order: 'tailwind-base, primevue, tailwind-utilities',
      },
    },
  },
})
app.use(ToastService)

app.mount('#app')

// eslint-disable-next-line no-console
console.info(`Hej Nathejk v${__APP_VERSION__}`)

// Register the service worker (installability + update detection). When a new
// build is waiting, flag it on the app store; UpdatePrompt (task 020) shows the
// reload affordance.
initPwa(() => {
  useAppStore().setUpdateAvailable(true)
})
