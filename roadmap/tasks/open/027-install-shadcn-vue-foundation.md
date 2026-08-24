# 027 — Install the shadcn-vue foundation (Tailwind v4)

**Status:** open
**Priority:** high
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `reka-ui`, `class-variance-authority`, `clsx`, `tailwind-merge` and
      `tw-animate-css` are installed; `tailwindcss-animate` is **not**.
- [ ] `vue/components.json` exists with aliases matching repo conventions
      (`cn()` under `@/helpers`, not `@/lib/utils`), `cssVariables: true`, and
      `lucide` as the icon library.
- [ ] `cn()` is implemented and exported from `@/helpers`.
- [ ] A trial `npx shadcn-vue@latest add button` writes to
      `src/components/ui/button/` and compiles (revert it if it is not part of
      task 029's chosen set).
- [ ] `npm run type-check` and `npm run build` pass in the `ui` container.
- [ ] `docker compose build ui` succeeds after the lockfile change.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
