import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import Components from 'unplugin-vue-components/vite'
import { PrimeVueResolver } from '@primevue/auto-import-resolver'
import { VitePWA } from 'vite-plugin-pwa'

// https://vite.dev/config/
export default defineConfig({
  define: {
    // Build/version id exposed to the app (npm sets npm_package_version).
    __APP_VERSION__: JSON.stringify(process.env.npm_package_version ?? 'dev'),
  },
  plugins: [
    vue(),
    // Tailwind v4 via its Vite plugin. Configuration is CSS-first in
    // @/assets/main.css (@import "tailwindcss", @theme) — there is no
    // tailwind.config.js, and no PostCSS config is needed.
    tailwindcss(),
    // Auto-import PrimeVue components (<Button>, <InputText>, …) — no manual imports.
    Components({
      resolvers: [PrimeVueResolver()],
      dts: 'src/components.d.ts',
    }),
    // PWA: installable to the home screen, standalone display. registerType
    // 'prompt' surfaces a "new version" event the app turns into an update
    // prompt (see @/helpers/pwa + task 020). Manifest values mirror
    // @/config/brand.
    VitePWA({
      registerType: 'prompt',
      injectRegister: false, // we register manually in @/helpers/pwa
      includeAssets: ['favicon.svg', 'logo.svg'],
      manifest: {
        name: 'Hej Nathejk',
        short_name: 'Hej Nathejk',
        description: 'Nathejk in-event companion — maps, contacts, rulebook and updates.',
        theme_color: '#0f172a',
        background_color: '#ffffff',
        display: 'standalone',
        start_url: '/',
        scope: '/',
        lang: 'da',
        icons: [
          { src: '/logo.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' },
          { src: '/logo.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'maskable' },
        ],
      },
      workbox: {
        // Pull in custom push / notificationclick handlers (public/push-sw.js).
        importScripts: ['push-sw.js'],
      },
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
    // Accept the proxied hostname(s) — Vite 5.4+ blocks unknown Host headers.
    allowedHosts: ['.local.nathejk.dk'],
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
