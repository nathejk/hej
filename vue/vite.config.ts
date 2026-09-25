import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'
import {
  GLIMT_MEDIA_CACHE_MAX_AGE_SECONDS,
  GLIMT_MEDIA_CACHE_MAX_ENTRIES,
  GLIMT_MEDIA_CACHE_NAME,
  GLIMT_MEDIA_URL_PATTERN,
  GLIMT_THUMB_CACHE_MAX_ENTRIES,
  GLIMT_THUMB_CACHE_NAME,
  GLIMT_THUMB_URL_PATTERN,
  PORTRAIT_CACHE_MAX_AGE_SECONDS,
  PORTRAIT_CACHE_MAX_ENTRIES,
  PORTRAIT_CACHE_NAME,
  PORTRAIT_URL_PATTERN,
  TILE_CACHE_MAX_AGE_SECONDS,
  TILE_CACHE_MAX_ENTRIES,
  TILE_CACHE_NAME,
  TILE_URL_PATTERN,
} from './src/config/cache'

// https://vite.dev/config/
export default defineConfig({
  define: {
    // Build/version id exposed to the app (npm sets npm_package_version).
    __APP_VERSION__: JSON.stringify(process.env.npm_package_version ?? 'dev'),
    // A build identity that actually differs between builds.
    //
    // __APP_VERSION__ cannot do this job: package.json's version is pinned at
    // 0.0.0 and nobody bumps it, so every build reports the same string — which
    // is worthless when the question is "is the phone running the build I just
    // deployed, or a service worker's stale copy?".
    //
    // BUILD_VERSION is the existing org convention (`<ref_name>.<run_number>`,
    // set by .github/workflows/build-and-publish.yml and passed to the image as a
    // Docker build arg — see docker/Dockerfile, where the Go binary already
    // consumes it via -ldflags). Reused here rather than introducing a second
    // scheme. Note the build context deliberately excludes .git, so deriving this
    // from a commit sha at build time is not an option.
    //
    // In dev there is no BUILD_VERSION, so it falls back to the moment the dev
    // server started — which is the useful signal locally, since that is what
    // changes when you restart it.
    __BUILD_ID__: JSON.stringify(
      process.env.BUILD_VERSION || `dev ${new Date().toISOString().slice(5, 16).replace('T', ' ')}`,
    ),
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
      includeAssets: ['favicon.svg', 'apple-touch-icon.png', 'badge-96.png'],
      manifest: {
        name: 'Hej Nathejk',
        short_name: 'Hej Nathejk',
        description: 'Nathejk in-event companion — maps, contacts, rulebook and updates.',
        theme_color: '#0f172a',
        // Matches theme_color rather than being white: this is the colour of the
        // synthesised launch screen, and white flashes on every cold start.
        background_color: '#0f172a',
        display: 'standalone',
        start_url: '/',
        scope: '/',
        lang: 'da',
        // PNG, not SVG. Android derives the launcher icon and the splash-screen
        // artwork from these, and the 'any'/'maskable' pair are genuinely
        // different framings (the maskable one is inset so the crescent's thin
        // horns survive the circle crop) — so they cannot be the same file, as
        // they were when both pointed at the placeholder logo.svg.
        //
        // Regenerate with vue/scripts/generate-icons.sh after editing the
        // vectors in src/assets/brand/.
        // Turns Chromium's bare install prompt (favicon + URL) into the richer
        // dialog with name, description and a preview. Regenerate with
        // vue/scripts/capture-screenshots.sh against a running dev stack.
        //
        // `sizes` must match each file exactly or the entire set is ignored, and
        // at least one `narrow` plus one `wide` entry is needed for the prompt to
        // upgrade. The narrow one is 540 wide rather than a true phone width
        // because headless Chrome cannot lay out below ~500 — see the script.
        //
        // Only the login view so far: every other route is behind the auth guard,
        // so maps/rulebook shots need a session for a seeded person.
        screenshots: [
          {
            src: '/screenshots/login-narrow.png',
            sizes: '540x1080',
            type: 'image/png',
            form_factor: 'narrow',
            label: 'Log ind med dit telefonnummer',
          },
          {
            src: '/screenshots/login-wide.png',
            sizes: '1280x800',
            type: 'image/png',
            form_factor: 'wide',
            label: 'Log ind med dit telefonnummer',
          },
        ],
        icons: [
          { src: '/pwa-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: '/pwa-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
          { src: '/maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        // Three path groups the navigation fallback must **not** answer for.
        //
        // `/desktop.html` is the desktop placeholder (task 140): a plain file, not part of this
        // app. Without the exclusion an installed client asking for it would be served
        // index.html, boot the app, and be redirected straight back — a loop, and the one
        // failure mode of moving that page out of the SPA.
        //
        // `/offentligt/` is the public Glimt page (PRD 019, task 323), server-rendered by the
        // BFF. This one is easy to get wrong because it fails *asymmetrically*: a parent following
        // a shared link has no service worker and gets the real page, so the bug is invisible to
        // everyone except **installed members**, who would get the app shell instead of the page
        // they clicked. Prefix rather than exact, so the page can gain siblings.
        //
        // **`/api/` is never a navigation destination**, and leaving it out cost a real bug
        // (task 318, found on a device 2026-09-17): tapping *Gem* in the viewer downloaded a 14 kB
        // file called `nathejk-….jpg.html`. Safari treats an `<a download>` click as a navigation,
        // the fallback answered it with index.html, and the member got the app shell renamed as a
        // photograph. Anything under `/api/` that a browser navigates to should get the API's own
        // answer — bytes, or a JSON 404 — never the shell.
        navigateFallbackDenylist: [
          /^\/desktop\.html$/,
          // The public site, which the app's fallback must never answer (PRD 011, task 332). It lives
          // under the **event year** (task 351), so the pattern matches a four-digit first segment
          // rather than a literal prefix: this bundle does not know which year the server serves, and
          // next year's deployment must not need a frontend change to stay out of the way.
          //
          // The bare path as well as everything under it: the prefix-with-slash pattern does not match
          // a path with no trailing slash, and that omission is exactly the bug task 332 shipped for
          // the site's first address — an installed member following a link to the frontpage got the
          // app shell.
          /^\/\d{4}$/,
          /^\/\d{4}\//,
          /^\/api\//,
        ],
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
            //
            // The matcher itself lives in src/config/cache.ts so a test can run it against the
            // patrol-lookup URLs (task 170). Safe to import here: `urlPattern` is evaluated at
            // build time, unlike the function bodies below.
            urlPattern: TILE_URL_PATTERN,
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
          {
            // Portrait thumbnails get their own cache (task 192, PRD 009 §8).
            //
            // They were already being cached by the browser's HTTP cache via
            // `Cache-Control: private, max-age=3600`, which is enough to draw a list and useless
            // for everything else: an HTTP cache cannot be measured for the readiness view,
            // cannot be evicted in a priority order, and cannot be purged after the event. A
            // named Cache API bucket can be all three.
            // `people`, anchored — **not** `contacts/.*\/photo`. The patrol lookup serves minors'
            // faces from `/api/contacts/patrols/{n}/photo/{id}` and must never be cached, so this
            // matcher lives in src/config/cache.ts with that reasoning next to it and a test that
            // runs it against those URLs (task 170).
            urlPattern: PORTRAIT_URL_PATTERN,
            // CacheFirst is safe *because* the URL carries a content hash (`?v=`): a changed
            // portrait is a different URL, so there is nothing to revalidate. The old entry then
            // ages out rather than being replaced, which the entry cap absorbs.
            handler: 'CacheFirst',
            options: {
              cacheName: PORTRAIT_CACHE_NAME,
              expiration: {
                maxEntries: PORTRAIT_CACHE_MAX_ENTRIES,
                maxAgeSeconds: PORTRAIT_CACHE_MAX_AGE_SECONDS,
                // Same reasoning as the tile cache: NOT purgeOnQuotaError. Losing every face
                // mid-race because one write did not fit is worse than not storing the newest.
              },
              // 200 only, and this one matters more than for tiles: PRD 007 makes the endpoint
              // return an indistinguishable 403/404 for "not allowed" and "no photo", and caching
              // either would freeze an authorization decision on the device for a fortnight — a
              // member whose role changes mid-event would keep seeing the refusal.
              cacheableResponse: { statuses: [200] },
            },
          },
          {
            // Glimt **thumbnails** — the grid tiles (PRD 019 §8, task 315).
            //
            // Matched by the `variant=thumb` query parameter, which is what distinguishes them from
            // full-size media on the same path. Workbox matches on the whole URL, so this route must
            // come **before** the full-media route below — the first matching route wins, and a
            // pattern without the parameter would swallow both.
            urlPattern: GLIMT_THUMB_URL_PATTERN,
            // CacheFirst. The bytes are immutable by construction: a different image is a different
            // content hash, so a URL's content can only change if the *glimt* changes, and a glimt's
            // media list is written once. The BFF says so too — `immutable` with a content-hash ETag
            // (task 305) — so there is nothing to revalidate against and SWR would re-request every
            // visible tile on every scroll, which is exactly the cost this exists to avoid.
            handler: 'CacheFirst',
            options: {
              cacheName: GLIMT_THUMB_CACHE_NAME,
              expiration: {
                maxEntries: GLIMT_THUMB_CACHE_MAX_ENTRIES,
                maxAgeSeconds: GLIMT_MEDIA_CACHE_MAX_AGE_SECONDS,
                // Not `purgeOnQuotaError`, for the reason the tile and portrait caches give:
                // losing every grid tile because one write did not fit is worse than not storing
                // the newest one.
              },
              // 200 only, and it matters here for the same reason as portraits: the media endpoint
              // answers 403 for "not shared with you" (task 305), and caching that would freeze an
              // authorization decision on the device for a fortnight — a member whose group changes
              // mid-event would keep seeing the refusal.
              cacheableResponse: { statuses: [200] },
            },
          },
          {
            // Glimt **full-size media** — what the viewer opens.
            //
            // A separate, much smaller cache: see the note on GLIMT_MEDIA_CACHE_MAX_ENTRIES. Its job
            // is to make going back to a photograph you just looked at instant, not to hold an
            // event. Sharing a cache with the thumbnails would let twenty opened photographs evict
            // several hundred grid tiles — the cheap, numerous, load-bearing things pushed out by
            // the expensive, rare ones.
            urlPattern: GLIMT_MEDIA_URL_PATTERN,
            handler: 'CacheFirst',
            options: {
              cacheName: GLIMT_MEDIA_CACHE_NAME,
              expiration: {
                maxEntries: GLIMT_MEDIA_CACHE_MAX_ENTRIES,
                maxAgeSeconds: GLIMT_MEDIA_CACHE_MAX_AGE_SECONDS,
              },
              cacheableResponse: { statuses: [200] },
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
      // The public, login-free pages (PRD 011) are server-rendered by the BFF,
      // not part of the SPA. They need proxying too, or they are unreachable in
      // a dev browser — the island script and vendored Leaflet under
      // vue/public/ are still served by Vite itself, same as in prod where the
      // Go binary serves both from ./www.
      //
      // Any four-digit year, not only this one: the curator's pages are served
      // for every workable year (task 392), so /2025/photos must reach the BFF
      // too. A key starting with `^` is a RegExp to Vite — the earlier comment
      // here said a key could not be a pattern, which is why it was hardcoded.
      '^/\\d{4}(/|$)': {
        target: 'http://api:4000',
        changeOrigin: true,
      },
      // The photographer admin tool (PRD 022) is server-rendered by the BFF too,
      // and needs proxying for the same reason the public pages do: without this
      // key Vite answers /admin with the SPA shell, so the tool is unreachable in
      // a dev browser and the door looks broken rather than unrouted.
      //
      // Worth knowing if it ever appears to be "200 but wrong": that is the
      // symptom of this key being absent, because the SPA fallback serves
      // index.html for any unmatched path. Production has no proxy and no such
      // failure — the Go binary serves both.
      //
      // A single key covers /admin and everything under it, since Vite matches
      // proxy keys as path prefixes.
      '/admin': {
        target: 'http://api:4000',
        changeOrigin: true,
      },
      // The shared photo viewer's two assets (PRD 023 §7.3, task 402), which the
      // public album page and the admin tool both load from `/viewer/<hash>/…`.
      //
      // Dev routing only. **The assets themselves are not here** — they live in
      // `go/cmd/api/viewer/` and are embedded in the Go binary, because the hard
      // rule in `.rules` is that website assets do not go under `vue/`. Without
      // this key Vite would answer them with the SPA shell, and a page would load
      // `index.html` as its JavaScript: the viewer would silently not open, which
      // looks like a viewer bug rather than an unrouted path.
      '/viewer': {
        target: 'http://api:4000',
        changeOrigin: true,
      },
    },
  },
})
