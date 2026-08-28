# 095 — Generate shadcn-vue `avatar` + `dropdown-menu` primitives

**Status:** done
**Priority:** high
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

PRD 003's user menu is an avatar trigger opening a menu; `.rules` requires a
standard shadcn-vue component whenever one exists, and neither primitive is in
`vue/src/components/ui/` yet (we have accordion, alert, button, card, dialog,
drawer, input, label, separator).

`dropdown-menu` is what gives us focus management, escape/outside-press
dismissal and roving-focus keyboard nav — the reason not to hand-roll a popover.
`avatar` gives the image/fallback swap the portrait-or-initials trigger needs.

Generated primitives are **owned source** (`.rules`): edited in place, not
wrapped.

## Acceptance Criteria

- [x] `vue/src/components/ui/avatar/` and `.../dropdown-menu/` generated via the
      shadcn-vue CLI, matching the existing primitives' structure and index
      re-exports.
- [x] No PrimeVue, no new icon set; any icons are Lucide.
- [x] `npm run type-check` clean. (`npm run lint` does not exist in this repo —
      `vue/package.json` has only `dev`, `build`, `preview`, `type-check`; the
      criterion was written from the PRD's assumption, not from the repo.)
- [x] No `tailwind.config.js` reintroduced by the CLI (Tailwind v4 is CSS-first
      in `src/assets/main.css`).

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Generated both primitives with `npx shadcn-vue@latest add avatar
  dropdown-menu` inside the `ui` container (22 files). The CLI picked up the
  existing `components.json`, so they landed on the `reka-vega` style, the `zinc`
  base colour and the repo's `@/components/ui` alias without configuration.
- 2026-08-28 — Verified the three icons the CLI wired in (`CheckIcon`,
  `ChevronRightIcon`) import from `@lucide/vue`, i.e. the CLI did **not** pull in
  `lucide-vue-next` as a second icon package. Confirmed no `tailwind.config.js`
  was created.
- 2026-08-28 — Note: the CLI also bumped `@lucide/vue` ^1.34→^1.35 and `reka-ui`
  ^2.10.3→^2.10.4 in `package.json`. Left in place: both are in-range patch/minor
  moves that `npm install` would have taken anyway, and pinning them back would
  desync `package-lock.json` from what was actually installed and type-checked.
- 2026-08-28 — ✅ All criteria met. `npm run type-check` clean. Moving to done.
