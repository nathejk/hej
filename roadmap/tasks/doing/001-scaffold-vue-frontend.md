# 001 — Scaffold Vue 3 frontend (`vue/`)

**Status:** doing
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:**

## Description

Create the Vue 3 (TS) SPA foundation under `vue/` following the
`vue3-spa-layout` conventions, as specified in
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. This is the base every
other frontend task builds on.

Stack: Vite, Tailwind, PrimeVue (unstyled + Lara preset), Pinia, `@/*` path
alias. Development happens inside the `ui` container — never on the host.

## Acceptance Criteria

- [ ] `vue/` created with `index.html`, `vite.config.*`, Tailwind + PostCSS
      config, `jsconfig`/`tsconfig`, `package.json` (dev/build/preview/test/lint
      scripts).
- [ ] `src/main.*` wires createApp + Pinia + PrimeVue (Lara preset) + router and
      mounts `#app`.
- [ ] `@` alias resolves to `vue/src/` in both Vite and jsconfig/tsconfig.
- [ ] `src/App.vue`, `src/router/index.ts`, and empty `src/views/`,
      `src/components/`, `src/stores/`, `src/helpers/`, `src/presets/lara/`
      folders exist.
- [ ] `docker compose run --rm ui npm run build` succeeds.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 16:00 — Picked up ("make runnable"). Plan: scaffold Vue 3 + **TypeScript** (per `.rules`/PRD) SPA under `vue/`: Vite + Tailwind 3 + Pinia + vue-router + PrimeVue 4 (`@primevue/themes` Lara) + primeicons + auto-import resolver; `@`→`src` alias; `main.ts`/`App.vue`/`router`/`HomeView`; `helpers` fetchWrapper; Vite dev server on :80 proxying `/api` → `http://api:4000`. Generate `package-lock.json` + verify `npm run build` inside a node container; then run ui+api on a shared docker network to confirm the proxy end-to-end (Traefik/DNS still needs the infra repo).
