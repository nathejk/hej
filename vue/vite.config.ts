import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'
import {
  TILE_CACHE_MAX_AGE_SECONDS,
  TILE_CACHE_MAX_ENTRIES,
  TILE_CACHE_NAME,
  TILE_HOST,
} from './src/config/cache'

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
        // Map tiles are cached as they are browsed (PRD 002 §11.2, task 087).
        //
        // This is the cheap half of offline maps and deliberately comes first: the bytes
        // are already being fetched to draw the map, so storing them costs nothing extra,
        // and a participant who looks at the map before the start arrives with that area
        // available offline without having agreed to a download. The expensive part — bulk
        // pre-fetching the whole race area, ~324 MB — is a separate, user-initiated
        // feature, because on iOS the app cannot tell WiFi from cellular
        // (`navigator.connection` is unavailable in Safari).
        runtimeCaching: [
          {
            // Host match rather than a path match: the three base layers live on three
            // different WMS endpoints of the same host, and a fourth may be added
            // (`topo_skaermkort`, PRD 002 §11).
            urlPattern: new RegExp(`^https://${TILE_HOST.replace(/\./g, '\\.')}/`),
            // CacheFirst, not StaleWhileRevalidate. Raster topographic maps change on a
            // scale of years, and the service sends no cache-control, etag or expires
            // header, so there is nothing to revalidate against — SWR would re-download
            // every visible tile on every view, which is exactly the mobile-data cost this
            // is meant to avoid.
            handler: 'CacheFirst',
            options: {
              cacheName: TILE_CACHE_NAME,
              expiration: {
                maxEntries: TILE_CACHE_MAX_ENTRIES,
                maxAgeSeconds: TILE_CACHE_MAX_AGE_SECONDS,
                // Deliberately NOT `purgeOnQuotaError: true`.
                //
                // That option deletes the *entire* cache when a write hits the quota, which
                // is the wrong trade here: a full cache that cannot grow is far better than
                // an empty one, and losing every tile mid-race is unrecoverable in the field
                // — offline, a purged tile cannot be re-fetched (PRD 009 §11.9). Without it,
                // an exhausted quota means the newest tiles simply are not stored while
                // everything already held keeps working.
                //
                // The related question — whether tiles should be sacrificed to protect
                // genuinely unrecoverable data like portraits — is a cross-dataset priority
                // decision and belongs to PRD 009 §11.1, not to this one cache's config.
              },
              // 200 only. An opaque response (status 0) is deliberately NOT cached: the
              // tile layer sets `crossOrigin`, and Dataforsyningen sends
              // Access-Control-Allow-Origin, so responses here are CORS-readable and count
              // toward quota at their real size. Accepting status 0 would let a
              // misconfiguration silently fill the cache with opaque responses, which
              // browsers pad heavily for quota accounting — turning a 324 MB budget into
              // something far larger for the same tiles.
              cacheableResponse: { statuses: [200] },
              // Normalise the cache key so the token and the retry counter are not part of
              // it. See TILE_CACHE_KEY_IGNORED_PARAMS in src/config/cache.ts for why.
              //
              // The parameter list is **inlined literally and must stay in sync with that
              // constant** — it cannot import it. Workbox's generateSW mode *stringifies*
              // this function into `sw.js` verbatim rather than bundling it, so any
              // identifier from the surrounding module scope becomes an undefined free
              // variable in the worker. Referencing the constant here built cleanly and
              // then threw `ReferenceError` on every tile request at runtime; only the
              // config values outside function bodies (cacheName, maxEntries, urlPattern)
              // are evaluated at build time and safely inlined.
              plugins: [
                {
                  cacheKeyWillBeUsed: async ({ request }: { request: Request }) => {
                    const url = new URL(request.url)
                    for (const param of ['token', '_retry']) {
                      url.searchParams.delete(param)
                    }
                    return url.href
                  },
                },
              ],
            },
          },
        ],
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
