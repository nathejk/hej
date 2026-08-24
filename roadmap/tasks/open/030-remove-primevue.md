# 030 — Remove PrimeVue

**Status:** open
**Priority:** high
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

## Description

Remove PrimeVue from the frontend entirely, per PRD 004. Deliberately sequenced
**last** among the code changes so the app is never left in a half-styled state.

This is cheap because **no PrimeVue component is actually used anywhere** in
`vue/src` — the library is only wired up, never consumed (the generated
`src/components.d.ts` lists only the app's own components). What has to go:

- `vue/package.json`: `primevue`, `@primevue/themes`,
  `@primevue/auto-import-resolver` (note `@primevue/themes` is deprecated
  upstream anyway).
- `vue/src/main.ts`: the `primevue/config`, `@primevue/themes/lara` and
  `primevue/toastservice` imports, plus the `app.use(PrimeVue, { theme: … })`
  and `app.use(ToastService)` calls and the `cssLayer` ordering block.
- `vue/vite.config.ts`: the `PrimeVueResolver` import and its use in
  `Components({ resolvers: [...] })` — the plugin itself is task 031.
- `main.css`: the `primevue` cascade layer, if task 024's rewrite has not already
  removed it.

`ToastService` is registered but unused. Do **not** replace it as part of this
task; whether we adopt `vue-sonner` is an open question on PRD 004 and should be
decided when something actually needs a toast.

PRD: 004. Depends on: 024, 027, 028, 029. Blocks: 031.

## Acceptance Criteria

- [ ] `primevue`, `@primevue/themes` and `@primevue/auto-import-resolver` are
      removed from `package.json` and `package-lock.json`.
- [ ] `main.ts` contains no PrimeVue imports, plugin registrations or theme
      config.
- [ ] `vite.config.ts` contains no reference to `PrimeVueResolver`.
- [ ] No `primevue` cascade layer remains in `main.css`.
- [ ] `grep -ri primevue vue/src vue/*.ts vue/*.json` (excluding the lockfile) is
      empty.
- [ ] `npm run type-check` and `npm run build` pass; every route renders with no
      console errors.
- [ ] `docker compose build ui` succeeds after the lockfile change.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
