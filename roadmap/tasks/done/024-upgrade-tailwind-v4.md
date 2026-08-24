# 024 — Upgrade Tailwind 3.4 → v4

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Upgrade the frontend from Tailwind 3.4 to the latest v4, per PRD 004. This is the
**first** task of the PRD and must land on its own commit so the codemod diff is
reviewable in isolation and any visual drift is attributable to this change rather
than to the shadcn-vue work that follows.

Current state: `vue/package.json` pins `tailwindcss@^3.4.15` with `autoprefixer` +
`postcss`, config in `vue/tailwind.config.js`, and `vue/src/assets/main.css` uses
`@tailwind` directives wrapped in a hand-declared cascade-layer order
(`@layer tailwind-base, primevue, tailwind-utilities`) that exists only to keep
PrimeVue below Tailwind utilities.

Approach:

1. Commit/stash everything first — `@tailwindcss/upgrade` wants a clean tree.
2. Run the official codemod in the container:
   `docker compose run --rm ui npx @tailwindcss/upgrade`
3. Review **every** diff by hand; the codemod is good but not total.
4. Switch to the `@tailwindcss/vite` plugin (preferred over
   `@tailwindcss/postcss`) and delete `postcss.config.js` + `autoprefixer` if
   nothing else needs them.
5. Move configuration into CSS-first `@theme` in `main.css` and delete
   `tailwind.config.js`.
6. Keep the `primevue` cascade layer out of the rewritten `main.css` — PrimeVue
   removal is task 030, but the v4 rewrite supersedes the layer hack regardless.

Watch for v4's changed defaults: border/ring colour and width, the shadow scale,
`outline-none` → `outline-hidden`, and `space-x/y` selector changes. The app's UI
is almost entirely utility classes (`BottomNav.vue`, `MoreMenu.vue`,
`PermissionPrompt.vue`, `UpdatePrompt.vue`, `PagePlaceholder.vue`,
`LoginView.vue`), so drift is likely — task 025 covers the visual pass.

All npm/npx work happens **inside the `ui` container**, never on the host.

PRD: 004. Blocks: 025, 027. Related: 026 (browser baseline).

## Acceptance Criteria

- [x] `vue/package.json` has `tailwindcss@^4` and no `autoprefixer`.
- [x] `@tailwindcss/vite` is registered in `vite.config.ts` (or
      `@tailwindcss/postcss` in `postcss.config.js`, with a note why).
- [x] `postcss.config.js` is deleted, or reduced to only what is still needed.
- [x] `vue/tailwind.config.js` is deleted (or reduced to the minimum v4 reads).
- [x] `main.css` uses `@import "tailwindcss"` with CSS-first `@theme`
      configuration; the `@tailwind` directive block and the `primevue` cascade
      layer are gone.
- [x] Dark mode is expressed as `@custom-variant dark`, not `darkMode: ['class']`.
- [x] The surviving global rules still apply: `html, body, #app { height: 100% }`,
      `body { margin: 0 }`, `color-scheme: light dark`.
- [x] `npm run build` and `npm run type-check` pass in the `ui` container.
- [x] `docker compose build ui` succeeds after the lockfile change.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 00:10 — Picked up. Plan: `npm ci` in the `ui` container, run
  `@tailwindcss/upgrade`, then hand-finish the config move (Vite plugin, CSS-first
  `@theme`, delete `tailwind.config.js`/`postcss.config.js`), then build +
  type-check.
- 2026-08-24 00:12 — Note on tooling: the `ui` image sets
  `ENTRYPOINT ["ui-dev"]`, so the documented `docker compose run --rm ui npm …`
  form does **not** work — the args are swallowed and the entrypoint runs
  `npm ci && npm run dev` instead. One-off commands need
  `docker compose run --rm --no-deps --entrypoint sh ui -c "…"`. Container is
  node 20.20.2 / npm 10.8.2.
- 2026-08-24 00:20 — Blocker: `npx @tailwindcss/upgrade` exited 1 with no output.
  Cause: it depends on `tree-sitter-javascript`, a native module that cannot build
  on `node:20-alpine` (no Python/toolchain — `gyp ERR! find Python`).
- 2026-08-24 00:24 — Blocker resolved: installed `python3 make g++` into the
  throwaway container before installing the codemod (`--no-save`, so nothing leaks
  into `package.json`; a later `npm ci` prunes it from the volume).
- 2026-08-24 00:28 — Codemod ran clean (v3.4.19 → **v4.3.3**): migrated
  `tailwind.config.js` into `main.css` and deleted it, migrated the stylesheet,
  bumped `tailwindcss`, removed `autoprefixer`, installed `@tailwindcss/postcss`
  and rewrote `postcss.config.js`. Templates: **0 files changed**.
- 2026-08-24 00:34 — Did **not** trust that "0 files changed". Audited every
  border/ring/shadow/outline/rounded/space utility in `src/**/*.vue` and checked
  the two risky ones against the installed `node_modules/tailwindcss/theme.css`
  rather than from memory:
  - **Borders: no risk.** Every border in the app names its colour
    (`border-slate-200`, `border-slate-300`), so v4's `currentcolor` default is
    irrelevant — I deleted the compat shim the codemod added rather than carrying
    dead cruft.
  - **Shadows: one real miss.** v4 renamed the scale, and `--shadow-xs` in v4 is
    v3's `shadow-sm` (`0 1px 2px 0 rgb(0 0 0/.05)`) while v4's `--shadow-sm` is
    v3's `shadow`. `PermissionPrompt.vue` used `shadow-sm` and would have silently
    gained a heavier shadow. Changed it to `shadow-xs`. The codemod missed this.
  - `shadow-lg` / `shadow-2xl` / all `rounded-{lg,xl,2xl,full}` are unchanged
    between v3 and v4; no bare `rounded`, no `ring` utility, no `space-x/y`,
    no opacity-suffix utilities. (The `ring` grep hits were Danish prose.)
- 2026-08-24 00:40 — Hand-finished the config move: switched to
  `@tailwindcss/vite` (faster, and lets the PostCSS config go entirely), deleted
  `postcss.config.js`, uninstalled `@tailwindcss/postcss` **and** `postcss`.
  Rewrote `main.css` to `@import 'tailwindcss'` at top level + `@custom-variant
  dark (&:is(.dark *))` for class-based dark mode (shadcn assumes it).
- 2026-08-24 00:42 — Nothing to port into `@theme`: the old `tailwind.config.js`
  was a stub with an empty `theme.extend` and no plugins. Left a comment marking
  where task 028's shadcn tokens go instead of an empty `@theme {}` block.
- 2026-08-24 00:44 — Left `main.ts`'s PrimeVue `cssLayer` config alone; it now
  names layers that no longer exist, which is harmless (it would just declare them
  empty, and no PrimeVue component is used anywhere). Task 030 deletes it.
- 2026-08-24 00:50 — ✅ Verified: `type-check` clean; `build` clean; `docker
  compose build ui` OK; `npm ci` from the rewritten lockfile is coherent. Also
  booted the **dev** server (the plugin swap affects dev, not just build) and
  fetched the compiled `main.css` — utilities generate, including `shadow-xs` and
  the arbitrary `pb-[env(safe-area-inset-bottom)]`.
- 2026-08-24 00:52 — Output CSS is 15.85 kB (gzip 3.99 kB); JS entry unchanged at
  266.15 kB (gzip 74.08 kB). No v3 baseline was captured before the upgrade, so
  the before/after comparison PRD 004 asks for is deferred to task 036. Expect v4
  CSS to be slightly larger (it emits its theme variable block), with the JS win
  coming from task 030's PrimeVue removal.
- 2026-08-24 00:54 — Unrelated finding for the backlog: `npm ci` warns
  `lucide-vue-next@0.454.0` is **deprecated in favour of `@lucide/vue`**. Not in
  scope here (`.rules` names `lucide-vue-next`), but it needs its own task.
- 2026-08-24 00:55 — All criteria met. Moving to done. Next: 025 (visual pass on a
  real viewport) and 026 (browser baseline — still the open question that could
  force a v3 rollback).
