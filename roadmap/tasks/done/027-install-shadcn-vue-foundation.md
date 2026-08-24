# 027 — Install the shadcn-vue foundation (Tailwind v4)

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Install the shadcn-vue foundation in the `vue/` workspace, per PRD 004. This is
plumbing only — component generation is task 029.

Runtime/dev dependencies (exact set follows the shadcn-vue installer for
Tailwind 4): `reka-ui`, `class-variance-authority`, `clsx`, `tailwind-merge`,
`tw-animate-css`. Note `tw-animate-css`, **not** the v3-era
`tailwindcss-animate` — installing the wrong one makes component animations
silently no-op.

`components.json` must be configured to fit **this repo's** conventions rather
than the CLI defaults:

- `aliases.components` → `@/components`
- `aliases.ui` → `@/components/ui`
- `aliases.utils` → the `cn()` helper under **`@/helpers`**, not the CLI-default
  `@/lib/utils` (shared utilities live in `helpers/` here)
- `tailwind.css` → `src/assets/main.css`, `tailwind.cssVariables` → true
- icon library → `lucide` (already a dependency; repo rule)
- TypeScript enabled

Export `cn()` from `@/helpers` and re-export it from `@/helpers/index.ts` so
imports match the existing `@/helpers` barrel pattern.

Verify `tsconfig.json`'s `@/*` path mapping satisfies the CLI — it resolves
`components.json` aliases through tsconfig when deciding where to write files.

All npm/npx work happens **inside the `ui` container**, never on the host. Files
the CLI writes must land in the mounted source tree, not the `ui-node_modules`
volume.

PRD: 004. Depends on: 024. Blocks: 028, 029.

## Acceptance Criteria

- [x] `reka-ui`, `class-variance-authority`, `clsx`, `tailwind-merge` and
      `tw-animate-css` are installed; `tailwindcss-animate` is **not**.
- [x] `vue/components.json` exists with aliases matching repo conventions
      (`cn()` under `@/helpers`, not `@/lib/utils`), `cssVariables: true`, and
      `lucide` as the icon library.
- [x] `cn()` is implemented and exported from `@/helpers`.
- [x] A trial `npx shadcn-vue@latest add button` writes to
      `src/components/ui/button/` and compiles (revert it if it is not part of
      task 029's chosen set).
- [x] `npm run type-check` and `npm run build` pass in the `ui` container.
- [x] `docker compose build ui` succeeds after the lockfile change.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 01:15 — Installed `reka-ui`, `class-variance-authority`, `clsx`,
  `tailwind-merge`, `tw-animate-css`. Wrote `src/helpers/utils.ts` with `cn()` and
  re-exported it from `@/helpers`.
- 2026-08-24 01:18 — Hand-wrote `components.json`… which turned out to be built on
  a **stale schema**: the current CLI uses styles named `vega`/`nova`/`maia`/…,
  not `new-york`. Deleted it and let `shadcn-vue init` generate the real thing,
  then re-applied our alias customisation.
- 2026-08-24 01:20 — **Blocker → task 038:** the CLI cannot run on Node 20 (an
  `undici` TypeError). Bumped the `ui` image to Node 22 first.
- 2026-08-24 01:35 — `init` run non-interactively:
  `--yes --base reka --style vega --icon-library lucide --base-color zinc --font inter`.
  Notes:
  - `--base-color slate` is rejected at runtime even though `--help` lists it;
    the registry only accepts `neutral|stone|zinc|mauve|olive|mist|taupe`. Chose
    **zinc** as the closest neutral to the app's slate.
  - The CLI is fully interactive without `--base`, `--style`, `--icon-library`
    and `--font`; `--yes` alone is not enough.
- 2026-08-24 01:40 — Corrected two things the CLI did against our conventions:
  - It wrote `src/lib/utils.ts` and pointed `aliases.utils` at `@/lib/utils`.
    Deleted `src/lib/`, pointed the aliases at `@/helpers/utils` + `@/helpers`.
    Verified by generating components: they import `from '@/helpers/utils'`.
  - It added `@import url('https://fonts.googleapis.com/…Inter…')` to `main.css`.
    **Removed.** A blocking third-party font request is the wrong trade for an app
    used on mobile data in rural areas, it leaks users to Google, and switching to
    Inter would be a visual change PRD 004 rules out. We stay on the system font
    stack (Tailwind's default `--font-sans` is untouched, so removing the import is
    all that is needed).
- 2026-08-24 01:45 — Also noted: `init` installed `@vueuse/core` (used by the
  generated components) and `@lucide/vue` — the latter collides with the
  deprecated `lucide-vue-next` we already had, which pulled task 037 forward.
- 2026-08-24 01:47 — ✅ type-check + build clean; `docker compose build ui` OK.
  Completed.
