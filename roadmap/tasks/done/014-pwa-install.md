# 014 — PWA install support (manifest, SW, icons, iOS meta)

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Make the app installable to the home screen and standalone, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`, using `vite-plugin-pwa`
(add via `docker compose run --rm ui npm install`). The service worker also
underpins push (task 016) and update detection (task 020).

Depends on: 001, 009 (branding/colors).

## Acceptance Criteria

- [x] `vite-plugin-pwa` configured; manifest with name "Hej Nathejk",
      short_name, icons (`logo.svg`, purpose any + maskable), `display:
      standalone`, theme/background colour, start_url, scope, lang.
- [x] Manifest auto-linked; iOS `apple-touch-icon` + `apple-mobile-web-app-*`
      (+ `mobile-web-app-capable`, title, status-bar-style) meta tags in
      `index.html`.
- [x] Service worker generated (`generateSW`); app shell precached (16 entries);
      registered via `@/helpers/pwa` (`registerType: 'prompt'`,
      `injectRegister: false`).
- [x] Build emits `sw.js` + `manifest.webmanifest`. *(Lighthouse install check
      requires the HTTPS deploy — infra Traefik/DNS — not runnable in this
      sandbox; the generated manifest/SW satisfy the installability criteria.)*

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 20:40 — Added `vite-plugin-pwa` (^0.21) + `VitePWA` config (manifest mirroring `@/config/brand`, SVG icons any+maskable, standalone). Added iOS/Apple + `mobile-web-app-capable` meta + `apple-touch-icon` to `index.html`. Created `@/helpers/pwa.ts` (`initPwa`/`applyUpdate` around `registerSW`) and wired `initPwa` in `main.ts` to set `app.store.updateAvailable` on `onNeedRefresh`. Added `vite-plugin-pwa/client` types to `env.d.ts`.
- 2026-07-30 20:41 — Decision: SVG icon (`logo.svg`) referenced with `purpose: any` + `maskable` instead of generating PNG sizes — keeps the skeleton dependency-light (no assets-generator); PNG sizes can be generated later if a target platform needs them. `injectRegister:false` + manual `registerSW` so the update flow (task 020) owns the lifecycle.
- 2026-07-30 20:42 — ✅ Verified in `node:20-alpine`: `npm run build` emits `dist/sw.js`, `dist/workbox-*.js`, `dist/manifest.webmanifest` (16 precache entries); `npm run type-check` clean.
- 2026-07-30 20:42 — Completed. Installable PWA + SW in place; `onNeedRefresh` already flips `app.store.updateAvailable` — task 020 adds the UpdatePrompt UI + `applyUpdate` wiring.
