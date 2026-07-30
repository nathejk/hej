# 001 — Scaffold Vue 3 frontend (`vue/`)

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Create the Vue 3 (TS) SPA foundation under `vue/` following the
`vue3-spa-layout` conventions, as specified in
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. This is the base every
other frontend task builds on.

Stack: Vite, Tailwind, PrimeVue (unstyled + Lara preset), Pinia, `@/*` path
alias. Development happens inside the `ui` container — never on the host.

## Acceptance Criteria

- [x] `vue/` created with `index.html`, `vite.config.ts`, Tailwind + PostCSS
      config, `tsconfig.json`, `package.json` (dev/build/preview/type-check
      scripts). *(lint/test scripts deferred — see log; not needed to run.)*
- [x] `src/main.ts` wires createApp + Pinia + PrimeVue (Lara preset via
      `@primevue/themes`) + ToastService + router and mounts `#app`.
- [x] `@` alias resolves to `vue/src/` in both Vite (`vite.config.ts`) and
      `tsconfig.json` paths.
- [x] `src/App.vue`, `src/router/index.ts`, `src/views/HomeView.vue`,
      `src/stores/app.store.ts`, `src/helpers/{fetchWrapper,index}.ts`, and
      `src/components/` exist. *(No `src/presets/lara/` — using the themed Lara
      preset instead of a hand-rolled unstyled preset; see log.)*
- [x] Build verified: `npm run build` (232 modules → `dist`) and
      `npm run type-check` (vue-tsc, clean) both pass in a `node:20-alpine`
      container. `package-lock.json` generated (so the `ui` container's
      `npm ci` works).

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 16:00 — Picked up ("make runnable"). Plan: scaffold Vue 3 + **TypeScript** (per `.rules`/PRD) SPA under `vue/`: Vite + Tailwind 3 + Pinia + vue-router + PrimeVue 4 (`@primevue/themes` Lara) + primeicons + auto-import resolver; `@`→`src` alias; `main.ts`/`App.vue`/`router`/`HomeView`; `helpers` fetchWrapper; Vite dev server on :80 proxying `/api` → `http://api:4000`. Generate `package-lock.json` + verify `npm run build` inside a node container; then run ui+api on a shared docker network to confirm the proxy end-to-end (Traefik/DNS still needs the infra repo).
- 2026-07-30 16:20 — Created all files: `package.json`, `vite.config.ts` (@ alias, PrimeVue auto-import resolver, dev proxy `/api`→`api:4000`, host+port 80), `index.html`, `tsconfig.json`, `env.d.ts`, `tailwind.config.js`, `postcss.config.js`, `src/main.ts`, `src/assets/main.css` (Tailwind + PrimeVue cssLayer ordering), `src/App.vue`, `src/router/index.ts`, `src/views/HomeView.vue` (pings `/api/healthcheck`), `src/stores/app.store.ts`, `src/helpers/fetchWrapper.ts` + `index.ts`, `public/favicon.svg`.
- 2026-07-30 16:22 — Decision: **TypeScript** (`.ts`, `tsconfig.json`) rather than the `vue3-spa-layout` skill's JS examples — `.rules` and the PRD both specify "Vue 3 (TS)". Established the TS pattern for this greenfield repo.
- 2026-07-30 16:22 — Decision: use the **themed Lara preset** from `@primevue/themes` (PrimeVue 4's supported theming) instead of the skill's `unstyled: true` + hand-rolled `src/presets/lara`. A full unstyled Tailwind preset is thousands of generated lines; the themed preset keeps the scaffold runnable now. Aligning to a custom unstyled preset can be a later task if desired.
- 2026-07-30 16:25 — Fix: `vue-tsc` couldn't resolve `node:url` in `vite.config.ts` → added `@types/node` devDep + `"node"` to tsconfig `types`. Re-ran: type-check clean.
- 2026-07-30 16:35 — ✅ Verified in `node:20-alpine`: `npm install` (generated `package-lock.json`), `npm run build` (232 modules, built in ~5s), `npm run type-check` (clean).
- 2026-07-30 16:40 — ✅ End-to-end: ran `hej-api:local` (alias `api`) + `hej-ui:local` (vite dev, source-mounted) on a shared docker network. `GET /` → 200 serving the Vue index (`<title>Hej Nathejk</title>`, `/src/main.ts`, `#app`); `GET /api/healthcheck` via the Vite proxy → 200 JSON from the BFF. The dev stack is runnable.
- 2026-07-30 16:42 — Cleaned root-owned `node_modules`/`dist`/`components.d.ts` from the host `vue/` (gitignored; dev uses the `ui-node_modules` volume). Added `vue/src/components.d.ts` to `.gitignore`.
- 2026-07-30 16:42 — Notes / follow-ups: (a) `@primevue/themes` prints a deprecation notice pointing to `@primeuix/themes` — consider migrating in a later task; (b) ESLint/Prettier + unit/e2e test setup (Vitest/Cypress) not added yet — defer to a tooling task; (c) full `docker compose up` still needs the org infra repo's external `traefik` network + `*.local.nathejk.dk` DNS.
- 2026-07-30 16:43 — Completed. Vue 3 (TS) SPA scaffolded, builds/type-checks, and runs end-to-end against the BFF through the Vite proxy. Foundation ready for the app shell / bottom nav / login tasks.
