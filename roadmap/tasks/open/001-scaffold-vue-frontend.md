# 001 — Scaffold Vue 3 frontend (`vue/`)

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
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
