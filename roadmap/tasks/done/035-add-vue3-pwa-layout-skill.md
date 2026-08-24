# 035 — Add the vue3-pwa-layout skill

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Create a new agent skill describing this repo's actual frontend stack, replacing
the legacy PrimeVue skill retired in task 034: Vue 3 + TypeScript, Tailwind v4,
shadcn-vue, Pinia, vue-router, installable PWA with service worker, Lucide icons.

Based on the legacy skill's structure, but rewritten for what the code really is
(TS not JS, PWA, no `src/presets/`, `src/config/`-driven navigation, and the
`type-check`/`build`-only script set) rather than what the sibling repos are.

Per the user's emphasis, the standard-component-first rule is the headline
section, placed before the directory layout so it cannot be skimmed past.

Completed ahead of PRD approval at the user's explicit request.

## Acceptance Criteria

- [x] `.agents/skills/vue3-pwa-layout/SKILL.md` exists with `name:` and a
      `description:` carrying trigger phrases for `vue/` work.
- [x] A prominent "The one rule that matters most" section states: use a standard
      shadcn-vue component whenever one exists; hand-roll only when absolutely
      necessary, and comment why. Includes the `add` command, the
      generate → compose → adapt-in-place workflow, a worked comment example, and
      the accessibility rationale (Reka UI brings focus/ARIA/scroll-lock).
- [x] Documents the real directory layout including `src/components/ui/` as owned
      source and `components.json`.
- [x] Documents Tailwind v4 CSS-first styling, `cn()`, and Lucide-only icons.
- [x] Documents routing (data-driven `config/navigation.ts`, guards, `meta`),
      Pinia conventions, and `fetchWrapper` usage.
- [x] Documents the PWA specifics: `registerType: 'prompt'` update flow,
      `public/push-sw.js`, secure-context requirement, and never precaching
      third-party tiles.
- [x] Documents that verification is manual (no test suite yet).
- [x] Documents the `ui`-container-only workflow.
- [x] Don'ts list bans PrimeVue, hand-rolling what shadcn provides, wrapping
      primitives to restyle them, reintroducing `tailwind.config.js`, and
      host-side npm.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004 (retroactively — work was done
  before the PRD was approved, at the user's request).
- 2026-08-24 00:00 — Wrote the skill. Put "The one rule that matters most" ahead of
  the layout section so the standard-component-first rule is the first thing read.
- 2026-08-24 00:00 — Also captured conventions that existed in the code but were
  written down nowhere: capability stores must degrade and never throw, BFF
  `snake_case` → `camelCase` mapping at the store boundary, `hej.<area>.<thing>`
  localStorage key namespacing, and a mobile-first section (≥44px targets,
  safe-area insets, shell owns full-height layout, Danish copy).
- 2026-08-24 00:00 — Added the "never precache third-party tiles/media" rule, which
  PRD 002's map tiles will need.
- 2026-08-24 00:00 — Completed.
