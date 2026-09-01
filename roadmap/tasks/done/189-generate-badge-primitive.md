# 189 — Generate the shadcn-vue `badge` primitive

**Status:** done
**Priority:** low
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

PRD 009 §7 needs `Badge` for dataset states in the readiness view (task 187). `card`,
`progress`, `alert`, `button` and the rest are already generated in
`vue/src/components/ui/`; `badge` is not.

Per `.rules`: prefer a standard shadcn-vue component whenever one exists, and generated
primitives are **owned source** — edit them in place rather than wrapping them to patch
styling. So generate it properly now rather than hand-rolling a `<span>` with classes in the
readiness view and leaving the next feature to do the same.

## Acceptance Criteria

- [x] `vue/src/components/ui/badge/` generated, matching the conventions of the neighbouring
      primitives (cva variants in `index.ts`, thin `.vue` reading them through `cn`).
- [x] Tailwind v4 CSS-first only — no `tailwind.config.js` reintroduced.
- [x] No PrimeVue anything (PRD 004).
- [x] Renders in the readiness view (task 187) for "Klar", "Mangler" and the rest.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — Picked up. Following the shape of the neighbouring alert/button primitives (cva variants in index.ts, thin .vue).
- 2026-09-01 — **Added a non-upstream `warning` variant**, and noted it as a local deviation in
  the file the way `button/index.ts` notes its touch-target bump. The readiness view has a
  genuinely three-way state — present, old, gone — and rendering "old" as `destructive` would tell
  a participant something is wrong when their map is merely a week stale. A variant rather than a
  one-off class, so the next surface with the same distinction does not invent a fourth colour.
- 2026-09-01 — ✅ Done alongside task 187, which is where it renders. `type-check` and
  `npm run build` clean.
