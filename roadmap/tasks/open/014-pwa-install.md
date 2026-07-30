# 014 — PWA install support (manifest, SW, icons, iOS meta)

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Make the app installable to the home screen and standalone, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`, using `vite-plugin-pwa`
(add via `docker compose run --rm ui npm install`). The service worker also
underpins push (task 016) and update detection (task 020).

Depends on: 001, 009 (branding/colors).

## Acceptance Criteria

- [ ] `vite-plugin-pwa` configured; manifest with name "Hej Nathejk",
      short_name, maskable icons for required sizes, `display: standalone`,
      theme/background color, start_url, scope.
- [ ] Manifest linked from `index.html`; iOS `apple-touch-icon` +
      `apple-mobile-web-app-*` meta tags present.
- [ ] Service worker registered; app shell precached.
- [ ] Lighthouse mobile PWA "installable" check passes; app launches standalone.

## Progress Log

- 2026-07-30 13:12 — Task created.
