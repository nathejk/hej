import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { PrimeVueResolver } from '@primevue/auto-import-resolver'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    // Auto-import PrimeVue components (<Button>, <InputText>, …) — no manual imports.
    Components({
      resolvers: [PrimeVueResolver()],
      dts: 'src/components.d.ts',
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Vite dev server listens on :80 inside the container (Traefik/EXPOSE 80).
    host: true,
    port: 80,
    // In dev, proxy the API to the Go BFF container. In prod the Go binary
    // serves both the API and the SPA from the same origin, so no proxy.
    proxy: {
      '/api': {
        target: 'http://api:4000',
        changeOrigin: true,
      },
    },
  },
})
