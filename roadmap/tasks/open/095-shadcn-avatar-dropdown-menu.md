# 095 — Generate shadcn-vue `avatar` + `dropdown-menu` primitives

**Status:** open
**Priority:** high
**Created:** 2026-08-28

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

- [ ] `vue/src/components/ui/avatar/` and `.../dropdown-menu/` generated via the
      shadcn-vue CLI, matching the existing primitives' structure and index
      re-exports.
- [ ] No PrimeVue, no new icon set; any icons are Lucide.
- [ ] `npm run type-check` and `npm run lint` clean.
- [ ] No `tailwind.config.js` reintroduced by the CLI (Tailwind v4 is CSS-first
      in `src/assets/main.css`).

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
