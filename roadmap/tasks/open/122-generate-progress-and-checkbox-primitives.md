# 122 — Generate the `progress` and `checkbox` shadcn-vue primitives

**Status:** open
**Priority:** medium
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §7. Two shadcn-vue primitives that PRD 005's onboarding flow needs are not yet
generated in `vue/src/components/ui/`:

- **`progress`** — the step indicator in the `/welcome` onboarding shell.
- **`checkbox`** — the *"Dette nummer kan kontaktes i løbet af Nathejk"* acknowledgement in
  the profile-confirmation step.

Everything else PRD 005 §7 calls for is already there: `card`, `button`, `input`, `label`,
`separator` and **`alert`** (the PRD's earlier note that `alert` was missing is out of date —
`vue/src/components/ui/alert` exists). So this task is exactly two primitives, no more.

Generate them per **PRD 004's on-demand rule**: primitives are added when a feature needs
them, not pre-generated as a set. Use the same CLI/flow that produced the existing components
so the output picks up the project's configured token/style setup, and check the result against
a neighbouring primitive — the generated files should look like siblings of what is already in
`vue/src/components/ui/`, not like a differently-configured import.

## They are owned source

Per `.rules`: generated primitives in `vue/src/components/ui/` are **owned source, edited in
place**. If the styling needs to change, change the primitive. Do **not** add a wrapper
component whose only job is to patch classes onto it — that is how a component library grows a
shadow layer nobody can reason about, and it defeats the reason shadcn-vue was chosen (PRD
004) over a dependency-shaped library.

Consequences to respect:

- Tailwind **v4**, configured CSS-first in `vue/src/assets/main.css`. If a generated file
  assumes a `tailwind.config.js`, adapt the file — do not reintroduce the config.
- Colours come from the existing design tokens (task 028), so no hardcoded palette values in
  the generated output.
- Icons are Lucide (`@lucide/vue`). `checkbox` ships with a check indicator — make sure it
  resolves to Lucide and not some other set.
- `reka-ui` is already a dependency and backs the existing primitives; these two should use it
  too rather than pulling in anything new.

This task only generates and lands the primitives. Wiring them into the onboarding steps
belongs to the views that consume them.

## Acceptance Criteria

- [ ] `vue/src/components/ui/progress/` and `vue/src/components/ui/checkbox/` exist, generated
      through the project's configured shadcn-vue flow
- [ ] Their structure and export style match the existing primitives in the same directory
- [ ] No `tailwind.config.js` is created or reintroduced; styling stays CSS-first in
      `vue/src/assets/main.css`
- [ ] Colours resolve through the existing design tokens — no hardcoded hex values
- [ ] `checkbox`'s indicator icon is Lucide; no new icon dependency
- [ ] No new runtime dependencies beyond what `reka-ui` already provides
- [ ] **No wrapper components** — any styling adjustment is made in the primitive itself
- [ ] Both render correctly in light and dark, and the checkbox is keyboard-operable and
      correctly labelled (it gates a step in a safety flow; an unreachable checkbox is a
      lockout)
- [ ] `npm run type-check` and `npm run build` clean
- [ ] No `primevue`, `@primevue/*` or PrimeIcons anywhere in the diff

## Progress Log

- 2026-08-30 — Task created from PRD 005.
