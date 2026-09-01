# 189 — Generate the shadcn-vue `badge` primitive

**Status:** open
**Priority:** low
**Created:** 2026-09-01

## Description

PRD 009 §7 needs `Badge` for dataset states in the readiness view (task 187). `card`,
`progress`, `alert`, `button` and the rest are already generated in
`vue/src/components/ui/`; `badge` is not.

Per `.rules`: prefer a standard shadcn-vue component whenever one exists, and generated
primitives are **owned source** — edit them in place rather than wrapping them to patch
styling. So generate it properly now rather than hand-rolling a `<span>` with classes in the
readiness view and leaving the next feature to do the same.

## Acceptance Criteria

- [ ] `vue/src/components/ui/badge/` generated, matching the conventions of the neighbouring
      primitives.
- [ ] Tailwind v4 CSS-first only — no `tailwind.config.js` is reintroduced.
- [ ] No PrimeVue anything (PRD 004).
- [ ] Renders in the readiness view for at least the "synced" and "mangler" states.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
