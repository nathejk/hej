# 009 — Branding "Hej Nathejk"

**Status:** done
**Priority:** medium
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Apply the "Hej Nathejk" brand across the app, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`: document title, PWA
manifest `name`/`short_name`, install prompt, home-screen label, and login
header. Theme + background colors feed the manifest and login screen.

Note: actual icon assets and final brand colors are an open item (see PRD
Decisions & Open Questions) — use placeholders and centralize the values so they
are easy to swap.

Depends on: 001.

## Acceptance Criteria

- [x] Document title and login header read "Hej Nathejk" (title in `index.html`;
      header/login now bind `APP_NAME`).
- [x] `APP_NAME`/`APP_SHORT_NAME` centralized in `@/config/brand`.
- [x] Theme/background colours defined in one place (`brand.ts`:
      `THEME_COLOR`/`BACKGROUND_COLOR`, matching the `index.html` `theme-color`)
      and ready to feed the PWA manifest in task 014.
- [x] Placeholder icon asset wired: `public/logo.svg` (512, maskable-friendly)
      as the single source task 014 will generate PWA icons from.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 20:20 — Added `@/config/brand.ts` (name/short/description/theme/background). Bound `APP_NAME` in `App.vue` top bar + `LoginView` header. Added `public/logo.svg` placeholder for icon generation. Colours already match the `index.html` `theme-color`.
- 2026-07-30 20:21 — ✅ Verified in `node:20-alpine`: build + type-check clean.
- 2026-07-30 20:21 — Completed. Brand values centralized; task 014 will consume `brand.ts` + `logo.svg` for the manifest/icons.
