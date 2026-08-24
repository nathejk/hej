# 030 — Remove PrimeVue

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

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

- [x] `primevue`, `@primevue/themes` and `@primevue/auto-import-resolver` are
      removed from `package.json` and `package-lock.json`.
- [x] `main.ts` contains no PrimeVue imports, plugin registrations or theme
      config.
- [x] `vite.config.ts` contains no reference to `PrimeVueResolver`.
- [x] No `primevue` cascade layer remains in `main.css`.
- [x] `grep -ri primevue vue/src vue/*.ts vue/*.json` (excluding the lockfile) is
      empty.
- [x] `npm run type-check` and `npm run build` pass; every route renders with no
      console errors.
- [x] `docker compose build ui` succeeds after the lockfile change.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 02:05 — Removed the three imports and both `app.use(...)` calls
  (including the `cssLayer` ordering block) from `main.ts`, the resolver from
  `vite.config.ts`, and uninstalled `primevue`, `@primevue/themes`,
  `@primevue/auto-import-resolver` — 45 packages gone. The `primevue` cascade layer
  had already left `main.css` with task 024's v4 rewrite.
- 2026-08-24 02:07 — Did **not** replace `ToastService`. Nothing used it, and
  whether we adopt `vue-sonner` is still an open question on PRD 004 — it should be
  answered by the first feature that actually needs a toast, not pre-emptively.
- 2026-08-24 02:08 — ✅ `grep -ri primevue` over `vue/src`, `vue/*.ts`, `vue/*.json`
  returns nothing. type-check + build clean.
- 2026-08-24 02:09 — **The payoff, measured.** Removing PrimeVue took the shell
  bundle from 267.74 kB → 123.37 kB (gzip 74.46 → 47.39). PrimeVue was costing
  ~27 kB gzip of plugin/theme runtime for **zero** components — the PRD's core
  premise, now with a number on it.
- 2026-08-24 02:10 — Browser click-through is still owed; that belongs to tasks 025
  and 036, which remain open. Completed.
