# 028 — Add shadcn design tokens to main.css

**Status:** open
**Priority:** high
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

## Description

Declare the shadcn-vue CSS-variable design tokens in `vue/src/assets/main.css`
using Tailwind v4's CSS-first mechanisms — `@theme inline` for the token →
utility mapping, `:root` for light values, `.dark` for dark values, and
`@custom-variant dark` for the variant — so generated components render with the
app's existing look.

The goal is **zero visual change**: map the tokens (`--background`,
`--foreground`, `--primary`, `--muted`, `--border`, `--ring`, `--radius`, …) onto
the palette the app already uses. The current UI is built on Tailwind's slate
scale (`border-slate-200`, `bg-white`, `text-slate-500`) with brand values in
`@/config/brand`.

Open question from PRD 004 to settle here and record: do we mirror the current
slate/Tailwind defaults to guarantee no visual change, or make the Nathejk brand
colours from `@/config/brand` the token source of truth? Recommendation: mirror
first (safe, reviewable), then rebrand deliberately in a later task.

Also decide whether to include the `.dark` block now. It is cheap and unused —
but note the app already sets `color-scheme: light dark`, which is arguably a
half-promise of dark mode. Shipping actual dark mode is **out of scope** for
PRD 004.

PRD: 004. Depends on: 024, 027. Blocks: 029.

## Acceptance Criteria

- [ ] Token variables are declared in `main.css` (`:root`, optional `.dark`) and
      exposed to utilities via `@theme inline`.
- [ ] `@custom-variant dark` is defined.
- [ ] `tw-animate-css` is imported.
- [ ] Token values reproduce the app's current colours; no visible change on any
      route.
- [ ] The token-palette decision (mirror current palette vs. adopt brand colours)
      is recorded in the progress log.
- [ ] The dark-block decision is recorded in the progress log.
- [ ] `npm run build` passes in the `ui` container.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
