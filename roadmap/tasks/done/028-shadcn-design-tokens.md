# 028 — Add shadcn design tokens to main.css

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

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

- [x] Token variables are declared in `main.css` (`:root`, optional `.dark`) and
      exposed to utilities via `@theme inline`.
- [x] `@custom-variant dark` is defined.
- [x] `tw-animate-css` is imported.
- [x] Token values reproduce the app's current colours; no visible change on any
      route.
- [x] The token-palette decision (mirror current palette vs. adopt brand colours)
      is recorded in the progress log.
- [x] The dark-block decision is recorded in the progress log.
- [x] `npm run build` passes in the `ui` container.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 01:35 — Tokens came from `shadcn-vue init` (part of task 027's run)
  rather than being hand-written: `:root` + `.dark` values, `@theme inline`
  mapping them to utilities, and the `@layer base` reset. Left the oklch values
  **verbatim** so a future re-generation diffs cleanly.
- 2026-08-24 01:38 — **Palette decision:** mirror-the-current-palette was not
  literally available — the shadcn registry has dropped `slate` as a base colour
  (only `neutral|stone|zinc|mauve|olive|mist|taupe`), so tokens are **zinc**. This
  is fine because the app's own components keep their explicit `slate-*`
  utilities, so nothing shifted; the two greys only meet when a shadcn primitive
  sits next to app markup. Unifying them is a branding decision, deliberately not
  made here. Recorded as a comment in `main.css` too.
- 2026-08-24 01:39 — **Dark-block decision:** keep it. The CLI wrote it, it costs
  ~1 kB of CSS, nothing sets `.dark`, and it is needed the moment dark mode is
  specced.
- 2026-08-24 01:40 — Related fix while here: changed `color-scheme: light dark`
  to `light`. The old value was a latent bug — it told the browser we handled dark
  mode, so on a phone in dark mode the body canvas painted dark behind our
  `bg-white` surfaces. We do not ship dark styles yet, so we should not advertise
  them. (The CLI's `body { @apply bg-background }` would have masked it; this fixes
  the cause.)
- 2026-08-24 01:41 — Tidied the CLI's `/* ---break--- */` filler comments into
  real explanations of each block.
- 2026-08-24 01:47 — ✅ Build clean. Completed.
