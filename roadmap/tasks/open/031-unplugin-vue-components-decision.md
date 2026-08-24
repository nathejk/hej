# 031 — Decide the fate of unplugin-vue-components

**Status:** open
**Priority:** medium
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

## Description

`unplugin-vue-components` is registered in `vue/vite.config.ts` **only** to host
`PrimeVueResolver`. Once PrimeVue is gone (task 030) its only remaining effect is
globally auto-registering the app's own components and generating
`src/components.d.ts`.

shadcn-vue uses explicit imports, and explicit imports are better for
readability and for tracing where a component is used. **Recommendation: remove
the plugin** and add explicit imports everywhere, deleting
`src/components.d.ts`.

The catch: removing global auto-registration produces *runtime* "failed to
resolve component" warnings that `vue-tsc` will **not** catch. Every template
must be checked for components used without an import — currently
`BottomNav`, `MoreMenu`, `PagePlaceholder`, `PermissionPrompt`, `UpdatePrompt`
(`RouterLink`/`RouterView` come from vue-router and are unaffected).

If we keep the plugin instead, leave a comment in `vite.config.ts` explaining why,
so the next reader does not assume it is PrimeVue leftovers.

PRD: 004. Depends on: 030.

## Acceptance Criteria

- [ ] Decision made and recorded in the progress log.
- [ ] If removed: `unplugin-vue-components` is uninstalled, gone from
      `vite.config.ts`, `src/components.d.ts` is deleted, and every component used
      in a template is explicitly imported.
- [ ] If kept: a comment in `vite.config.ts` states why, and it no longer
      references any PrimeVue resolver.
- [ ] Every route is loaded in the browser and the console shows **no** "failed to
      resolve component" warnings.
- [ ] `npm run type-check` and `npm run build` pass in the `ui` container.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
