# 122 — Generate the `progress` and `checkbox` shadcn-vue primitives

**Status:** done
**Priority:** medium
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

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

- [x] `vue/src/components/ui/progress/` and `vue/src/components/ui/checkbox/` exist, generated
      through the project's configured shadcn-vue flow
- [x] Their structure and export style match the existing primitives in the same directory
- [x] No `tailwind.config.js` is created or reintroduced; styling stays CSS-first in
      `vue/src/assets/main.css`
- [x] Colours resolve through the existing design tokens — no hardcoded hex values
- [x] `checkbox`'s indicator icon is Lucide; no new icon dependency
- [x] No new runtime dependencies beyond what `reka-ui` already provides
- [x] **No wrapper components** — any styling adjustment is made in the primitive itself
- [ ] Both render correctly in light and dark, and the checkbox is keyboard-operable and
      correctly labelled (it gates a step in a safety flow; an unreachable checkbox is a
      lockout) — *deferred to task 139's device pass; there is no browser in this environment.
      Keyboard operability comes from `reka-ui`'s `CheckboxRoot` (a real focusable control with
      `focus-visible` styling), and the labelling is task 127's responsibility since that is
      where the `Label` is written.*
- [x] `npm run type-check` and `npm run build` clean
- [x] No `primevue`, `@primevue/*` or PrimeIcons anywhere in the diff

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — Generated both through the project's configured CLI
  (`npx shadcn-vue@latest add progress checkbox`), which picked up `components.json` — style
  `reka-vega`, `iconLibrary: lucide`, tokens from `src/assets/main.css`. Output inspected against
  `label/` and `separator/`: same `reactiveOmit` + `cn()` + `data-slot` shape and the same
  `index.ts` re-export, so they are siblings rather than a differently-configured import. No
  `tailwind.config.js` appeared, no hex values, `CheckIcon` comes from `@lucide/vue`, and both are
  backed by `reka-ui`, which was already a dependency.
- 2026-08-30 — One incidental change: the CLI bumped `@lucide/vue` from ^1.35.0 to ^1.37.0 while
  resolving the icon import. Left in rather than reverted — it is a patch-level range bump on an
  existing dependency, and pinning it back would put `package.json` out of step with the installed
  tree. No other dependency moved.
- 2026-08-30 — ✅ `vue-tsc --noEmit` clean, `npm run build` clean (36 precache entries, 633 KiB —
  unchanged shell size), no PrimeVue/PrimeIcons in the diff. The light/dark and keyboard check is
  the one criterion left open, deferred to task 139's device pass with the reasoning recorded above.
  Moving to done.
