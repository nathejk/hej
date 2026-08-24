# PRD 004 — Migrate the component library from PrimeVue to shadcn-vue (and upgrade Tailwind to v4)

**Status:** doing
**Author:** agent session (Zed / Claude Opus 5)
**Created:** 2026-08-24
**Last updated:** 2026-08-24 (implementation notes + resolved questions)
**Approved:** 2026-08-24
**Shipped:**
**Target users:** none directly — developer-facing foundation change; all app roles benefit indirectly

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See the `prd` skill for the lifecycle.
-->

---

## 1. Summary

Replace PrimeVue with [shadcn-vue](https://www.shadcn-vue.com/) as the component
library for the `vue/` frontend: remove the PrimeVue runtime, theme preset and
auto-import resolver, upgrade Tailwind from 3.4 to the latest v4, install the
shadcn-vue foundation (Reka UI + `cn()` + CSS-variable theme tokens), and
generate the primitives the app needs today. From then on, shadcn-vue's standard
components are the default choice for all UI, and PrimeVue must not be
reintroduced — both recorded as rules in `.rules` and in a new `vue3-pwa-layout`
skill.

## 2. Problem & Motivation

- **What problem does this solve?** The frontend currently pays for PrimeVue
  without using it. PrimeVue is installed and wired up in `vue/src/main.ts`
  (`PrimeVue` plugin with the `Lara` theme preset and `ToastService`) and its
  auto-import resolver runs in `vue/vite.config.ts`, but **not a single PrimeVue
  component is used anywhere in `vue/src`** — the generated
  `vue/src/components.d.ts` lists only the app's own components, and every piece
  of UI shipped in PRD 001 (`BottomNav.vue`, `MoreMenu.vue`,
  `PermissionPrompt.vue`, `UpdatePrompt.vue`, `PagePlaceholder.vue`,
  `LoginView.vue`) is hand-rolled Tailwind. What we have is the cost of a
  component library — three dependencies, a CSS-layer ordering workaround in
  `main.css`/`main.ts`, and a deprecated transitive package (`@primevue/themes`
  is marked "no longer maintained; migrate to `@primeuix/themes`") — with none of
  the benefit.
- **Why now?** This is the cheapest possible moment. The app has no PrimeVue
  usage to rewrite, so the "migration" is a removal plus a foundation install.
  Every week we wait, the two feature PRDs in flight (PRD 002 map overlays and
  bottom sheet; PRD 003 profile rows, dialogs and capture surface) add more
  hand-rolled UI or, worse, start reaching for PrimeVue and create real
  migration debt. Both PRDs explicitly need dialog/sheet/list primitives — they
  should be built on the library we intend to keep.
- **Evidence.**
  - `grep` for PrimeVue component usage across `vue/src` returns only the four
    wiring lines in `main.ts`.
  - `@primevue/themes` is deprecated upstream.
  - The app already standardises on **Lucide** icons (`.rules`), which is
    shadcn-vue's default icon set — while PrimeVue's native idiom is PrimeIcons,
    which `.rules` forbids. The current library fights the repo's own
    conventions.
  - shadcn-vue's model (copy the component source into the repo, own it, style it
    with Tailwind) matches how this codebase already writes UI, and avoids the
    "don't override the preset in components" friction the legacy frontend skill
    has to warn about.
  - **Tailwind is a version behind.** The repo pins Tailwind 3.4 while v4 is
    current, and shadcn-vue's documentation, installer and generated components
    now lead with v4 (CSS-first `@theme` configuration, `@tailwindcss/vite`).
    Generating shadcn components against v4 and then hand-porting them back to a
    v3 config means fighting the tool on every future `add`. Doing both moves at
    once means touching `main.css`, `tailwind.config.js` and `postcss.config.js`
    exactly once instead of twice — and since PrimeVue's cascade-layer workaround
    in `main.css` is being deleted anyway, the two changes overlap in the same
    files. Doing them separately would be more work, not less.

## 3. Goals

- The frontend has exactly one component-library approach, and it is shadcn-vue.
- PrimeVue and its theme/resolver packages are fully removed — dependencies,
  wiring, CSS layer workaround and generated types.
- The frontend runs the latest Tailwind (v4), configured the way shadcn-vue
  expects, so `npx shadcn-vue add <component>` produces code that works without
  hand-porting.
- The primitives the app needs now (button, input, dialog/sheet, and the
  supporting theme tokens) exist as owned components under
  `vue/src/components/ui/`.
- Using a **standard shadcn-vue component is the default**; hand-rolling UI
  becomes the documented exception, not the habit.
- No visual or behavioural regression in the shipped UI, and no increase in
  initial bundle size.
- The decision is written down where agents and developers will actually read it
  (`.rules`, plus a new frontend skill), so nobody reintroduces PrimeVue or
  reinvents a primitive that already exists.

## 4. Non-Goals

- **Redesigning the app.** This is a library swap, not a visual refresh. Any
  restyling is out of scope and belongs in its own PRD.
- **Rewriting the existing hand-rolled components** (`BottomNav.vue`,
  `MoreMenu.vue`, `PermissionPrompt.vue`, `UpdatePrompt.vue`,
  `PagePlaceholder.vue`) onto shadcn primitives wholesale. They work. Refactor
  them opportunistically, only where a primitive is a clear win (see
  Requirements for the one candidate: `MoreMenu` → `Sheet`/`Drawer`).
- **Generating the full shadcn-vue catalogue.** Add components on demand, when a
  feature needs one.
- **Adopting new Tailwind 4 features beyond what the upgrade requires.** The
  upgrade is a migration, not an invitation to rewrite existing utility classes
  into container queries and `@theme` cleverness.
- **Introducing a dark theme.** We install the CSS-variable token layer that
  makes dark mode possible, but shipping dark mode is out of scope.
- **Changing Pinia, the router, `fetchWrapper`, or any BFF code.** This PRD
  touches `vue/` and documentation only. **No API changes, so no new OpenAPI
  annotations are required.**
- **A design-system documentation site / Storybook.**

## 5. User Stories & Scenarios

- As a **developer/agent** building the PRD 002 map overlays and PRD 003 profile
  page, I want an accessible dialog, sheet and button in the repo so that I do not
  hand-roll focus traps and ARIA wiring per feature.
- As a **developer/agent**, I want one obvious answer to "which component library
  does this repo use?" so that I do not have to infer it from a half-wired
  `main.ts`.
- As a **developer/agent**, I want component source in the repo so that I can
  restyle a component by editing it, instead of learning a preset override system.
- As an **end user**, I want the app to look and behave exactly as before, and
  load no slower.

**Primary happy path (the migration itself)**

1. Tailwind is upgraded 3.4 → latest v4: `@tailwindcss/vite` (or
   `@tailwindcss/postcss`) replaces the PostCSS/autoprefixer pipeline, and
   configuration moves from `tailwind.config.js` into CSS-first `@theme` blocks
   in `main.css`. The official `@tailwindcss/upgrade` codemod does the first pass.
2. PrimeVue packages are uninstalled and the wiring is removed from `main.ts`,
   `vite.config.ts` and `main.css` (including the `primevue` cascade layer, which
   the v4 rewrite of `main.css` supersedes anyway).
3. shadcn-vue is initialised for Tailwind v4: `components.json`, the `cn()`
   utility, and the CSS-variable design tokens as `@theme`/`:root` declarations
   with `@custom-variant dark` in place of `darkMode: ['class']`.
4. The needed primitives are generated into `vue/src/components/ui/`.
5. The app builds, type-checks, and renders identically; `MoreMenu` is optionally
   refactored onto the sheet primitive.
6. The legacy frontend skill is renamed to `vue3-spa-layout-legacy`, a new
   `vue3-pwa-layout` skill describes the current stack, and `.rules` states
   shadcn-vue as the library, standard components as the default, and PrimeVue as
   forbidden.

**Edge cases and risks in the migration**

- **CSS layer ordering:** `main.css` currently declares
  `@layer tailwind-base, primevue, tailwind-utilities` purely to keep PrimeVue's
  generated styles below Tailwind utilities. With PrimeVue gone and v4's
  `@import "tailwindcss"` replacing the `@tailwind` directives, this whole block
  disappears — but the existing cascade (notably the `html, body, #app` height
  rules and `color-scheme`) must survive the rewrite.
- **Tailwind 4 breaking changes:** removed/renamed utilities (e.g. `shadow-sm` →
  `shadow-xs` shifts, `outline-none` → `outline-hidden`), default border/ring
  colour and width changes, `space-x/y` selector changes, and the removal of
  implicit `preflight` opt-outs. The app's hand-rolled UI leans heavily on
  utilities (borders, rings, shadows on `BottomNav`, `MoreMenu`,
  `PermissionPrompt`, `UpdatePrompt`, `LoginView`), so a visual diff pass is
  mandatory rather than optional.
- **PostCSS pipeline:** v4 no longer needs `autoprefixer` or `postcss-import`;
  `postcss.config.js` either shrinks to `@tailwindcss/postcss` or is deleted
  entirely in favour of the `@tailwindcss/vite` plugin. Leaving stale plugins
  installed is a silent-failure risk.
- **`tailwindcss-animate` is v3-era:** the v4/shadcn answer is `tw-animate-css`.
  Installing the wrong one produces components whose animations silently no-op.
- **Browser support:** Tailwind v4 requires modern browsers (Safari 16.4+,
  Chrome 111+). This is a phone PWA whose users may run older iOS; the baseline
  needs an explicit decision (see Open Questions).
- **`unplugin-vue-components`:** it exists in `vite.config.ts` *only* to host
  `PrimeVueResolver`. shadcn-vue uses explicit imports, so the plugin (and the
  generated `src/components.d.ts`) should go — but the app's own components are
  currently registered globally by it, so any component used without an import
  must be found and imported explicitly, or the plugin kept resolver-less.
- **Alias convention clash:** the shadcn CLI defaults to `@/lib/utils`, while this
  repo puts utilities in `@/helpers`. `components.json` must be configured to
  match the repo, not the other way round.
- **Toast:** `ToastService` is registered but unused. shadcn-vue has no
  PrimeVue-equivalent service; the ecosystem answer is `vue-sonner`. Do not
  install it speculatively (see Open Questions).
- **Node version / CLI:** the shadcn-vue CLI and the Tailwind codemod must be run
  **inside the `ui` container** (`docker compose run --rm ui npx
  shadcn-vue@latest …`), never on the host, per the docker dev stack rules. Files
  they write must land in the mounted source tree, not the `ui-node_modules`
  volume. The codemod also wants a clean git tree, so commit before running it.
- **Lockfile churn:** removing three packages, upgrading Tailwind and adding
  several new ones rewrites `package-lock.json`; `docker compose build ui` is
  needed afterwards.

## 6. Requirements

### Functional

- [ ] `primevue`, `@primevue/themes` and `@primevue/auto-import-resolver` are
      removed from `vue/package.json` and `package-lock.json`.
- [ ] `vue/src/main.ts` no longer imports or registers `PrimeVue`, the `Lara`
      preset, or `ToastService`.
- [ ] `vue/vite.config.ts` no longer references `PrimeVueResolver`; the
      `unplugin-vue-components` plugin is removed or kept only with a documented
      reason, and `src/components.d.ts` is removed if the plugin goes.
- [ ] **Tailwind is upgraded from 3.4 to the latest v4** (`tailwindcss@^4`), with:
      - `@tailwindcss/vite` added to `vite.config.ts` (preferred) or
        `@tailwindcss/postcss` in `postcss.config.js`;
      - `autoprefixer` (and any other now-redundant PostCSS plugins) removed, and
        `postcss.config.js` deleted if `@tailwindcss/vite` makes it empty;
      - `main.css` migrated to `@import "tailwindcss"` with CSS-first `@theme`
        configuration, and the `@tailwind`/`@layer` directive block plus the
        `primevue` cascade layer removed;
      - `tailwind.config.js` removed, or reduced to only what v4 still needs;
      - dark mode expressed as `@custom-variant dark` rather than
        `darkMode: ['class']`.
- [ ] The existing global styles (`html, body, #app { height: 100% }`,
      `body { margin: 0 }`, `color-scheme`) still apply after the CSS rewrite.
- [ ] shadcn-vue is initialised **for Tailwind v4** with a `vue/components.json`
      whose aliases match repo conventions (`@/components`, `@/components/ui`, and
      the `cn()` helper under `@/helpers` rather than the CLI-default
      `@/lib/utils`), and `tailwind.cssVariables` enabled.
- [ ] The shadcn foundation is in place: `reka-ui`,
      `class-variance-authority`, `clsx`, `tailwind-merge`, `tw-animate-css`
      (the v4 successor to `tailwindcss-animate`), a `cn()` utility, and the
      CSS-variable design tokens.
- [ ] Lucide (`lucide-vue-next`, already a dependency) is the configured icon
      library for generated components — consistent with `.rules`.
- [ ] The initial component set is generated into `vue/src/components/ui/`,
      scoped to what is needed now and by the in-flight PRDs: at minimum
      `button`, `input`, `sheet` (or `drawer`), `dialog`, `card` and `separator`.
- [ ] `MoreMenu.vue`'s bottom sheet is evaluated against the generated
      `sheet`/`drawer` primitive and refactored onto it if that removes
      hand-rolled focus/ARIA/scroll-lock handling without changing appearance.
- [ ] The design tokens are set so the app's existing look (slate palette, brand
      colours from `@/config/brand`) is preserved.
- [ ] **Standard-component-first is documented as a rule:** when UI is needed, an
      existing shadcn-vue component is used; hand-rolling is allowed only when no
      shadcn component covers the need, and the reason is recorded in a comment on
      the component.
- [ ] `npm run build` and `npm run type-check` pass in the `ui` container.
- [ ] `.rules` states that this repo uses shadcn-vue, that standard shadcn
      components are preferred over hand-rolled ones, and that PrimeVue (and
      PrimeIcons) must not be used or reintroduced.
- [ ] The frontend skill is split: `.agents/skills/vue3-spa-layout/` is renamed to
      `.agents/skills/vue3-spa-layout-legacy/` (kept, clearly marked as describing
      the older PrimeVue-based sibling repos and **not** to be applied here), and a
      new `.agents/skills/vue3-pwa-layout/` skill describes this repo's stack:
      Vue 3 + TS, Tailwind v4, shadcn-vue, Pinia, PWA/service worker, Lucide,
      standard-component-first.

### Non-Functional

- **Bundle size:** the initial JS/CSS payload must not grow. Removing PrimeVue
  should shrink it; record before/after numbers in the task log.
- **Accessibility:** generated components are Reka UI based and keyboard/ARIA
  correct out of the box; the migration must not regress the existing
  `aria-label`s and touch-target sizes.
- **Mobile-first:** the app is a phone PWA. Generated components must be checked
  for ≥44px touch targets and safe-area compatibility, and adjusted in-repo where
  they are desktop-biased.
- **No behavioural change:** login flow, nav, overflow sheet, permission prompts
  and the update prompt behave identically after the swap.
- **Dependency hygiene:** no deprecated packages introduced; the Tailwind and
  shadcn-vue setups must match each other's current major (v4), so the CLI's
  generated output needs no hand-porting.
- **Browser baseline:** Tailwind v4 drops support for older engines (Safari <16.4,
  Chrome <111). The baseline must be stated explicitly and be acceptable for the
  phones event participants actually use.
- **Maintainability:** generated component source is committed and owned by us;
  local modifications get a short comment explaining the deviation from upstream
  so future re-generation is a conscious act.

## 7. UX / UI Notes

No intentional user-visible change. The success condition is that a user cannot
tell the migration happened.

Component-authoring convention going forward:

- **`src/components/ui/`** — shadcn-vue generated primitives. Treated as owned
  source: edit them directly rather than wrapping them to fix styling. Do not
  reformat them gratuitously, so upstream diffs stay legible.
- **`src/components/`** — app components, **composed from the primitives**. Reach
  for a standard shadcn-vue component first; hand-roll only when the catalogue has
  nothing that fits, and say why in a comment.
- **`src/views/`** — unchanged, one file per route, `*View.vue`.
- **Styling** — Tailwind utilities remain the primary tool; the shadcn CSS
  variables (now declared via Tailwind v4 `@theme`) become the source of truth for
  colour so themeing is central.
- **Icons** — Lucide only, unchanged.
- The obsolete `src/presets/` idea (never created in this repo, but prescribed by
  the legacy frontend skill) disappears along with `tailwind.config.js`.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):** the whole of this PRD lands in `vue/` plus two docs
  files. Files touched:
  - `vue/package.json`, `vue/package-lock.json` — dependency swap.
  - `vue/src/main.ts` — drop three imports and two `app.use(...)` calls.
  - `vue/vite.config.ts` — drop `PrimeVueResolver` (and probably `Components`).
  - `vue/src/assets/main.css` — rewritten for v4: `@import "tailwindcss"`,
    `@import "tw-animate-css"`, `@custom-variant dark`, shadcn token variables on
    `:root` (and `.dark`, unused for now) exposed through `@theme inline`, plus the
    surviving global rules. The `primevue` cascade layer goes.
  - `vue/tailwind.config.js` — deleted (v4 is CSS-first) or reduced to the
    minimum v4 still reads.
  - `vue/postcss.config.js` — deleted if `@tailwindcss/vite` is used; otherwise
    reduced to `@tailwindcss/postcss` with `autoprefixer` removed.
  - `vue/tsconfig.json` — verify the `@/*` path mapping satisfies the shadcn CLI
    (it writes based on `components.json` aliases resolved through tsconfig).
  - `vue/components.json` — new, CLI config.
  - `vue/src/components/ui/**` — new, generated.
  - `vue/src/helpers/` — `cn()` utility exported here (repo convention is that
    shared utils live in `helpers`), re-exported from `@/helpers/index.ts`.
  - `vue/src/components.d.ts` — deleted with the auto-import plugin.
  - `.rules`, `.agents/skills/vue3-spa-layout-legacy/` (renamed),
    `.agents/skills/vue3-pwa-layout/` (new) — documentation.
- **Tooling:** all `npm`/`npx` work happens in the `ui` container
  (`docker compose run --rm ui …`); `docker compose build ui` after the lockfile
  changes. Never on the host.
- **BFF (Go):** untouched.
- **API endpoints:** none added or changed. **No OpenAPI work required** — noted
  explicitly because the repo rule mandates annotations for any endpoint change,
  and there are none here.
- **Data / storage:** N/A — no persistence involved.
- **Interaction with in-flight PRDs:** PRD 002 (map) and PRD 003 (profile) both
  need overlay/sheet/dialog primitives. This PRD should land **before** their
  UI tasks so those features build on shadcn from the start; if they start first,
  they must not introduce PrimeVue components. Ordering is the main coordination
  risk.
- **Dependencies & risks:**
  - New: `tailwindcss@^4` + `@tailwindcss/vite` (or `@tailwindcss/postcss`),
    `reka-ui`, `class-variance-authority`, `clsx`, `tailwind-merge`,
    `tw-animate-css` (exact set follows the shadcn-vue installer for Tailwind 4).
    Removed: `primevue`, `@primevue/themes`, `@primevue/auto-import-resolver`,
    `autoprefixer`, `tailwindcss@3`, and possibly `postcss` and
    `unplugin-vue-components`.
  - **Tailwind v4 is a breaking major.** Run `npx @tailwindcss/upgrade` in the
    `ui` container from a clean tree as the first pass, then review every diff by
    hand — the codemod is good but not total, and the app's UI is almost entirely
    utility classes. Removed/renamed utilities and changed defaults (border and
    ring colour/width, shadow scale, `outline-none` → `outline-hidden`) are the
    likely sources of subtle visual drift.
  - Tailwind v4's browser baseline (Safari 16.4+, Chrome 111+) is a real
    constraint for a PWA used on whatever phone a participant owns.
  - shadcn-vue is a code-generation tool, not a versioned runtime: there is no
    upgrade path for generated components beyond re-running the CLI and diffing.
    That is the accepted trade-off for owning the source, but it means local edits
    should be documented.
  - Reka UI (the Radix-Vue successor) is a real runtime dependency and the actual
    accessibility engine; its maturity is the main technical bet.
  - Dropping global component auto-registration can produce runtime "failed to
    resolve component" errors that TypeScript will not catch — a build plus a
    click-through of every route is required, not just `type-check`.
  - Theme-token changes touch global colour; a careless mapping can shift existing
    shades subtly across the app.
  - The service worker precaches the CSS bundle; a Tailwind rewrite changes every
    asset hash, so the first deploy after this lands should be verified through the
    existing update-prompt flow rather than assumed.
  - There is no unit or e2e test suite in this repo yet (`vue/package.json` has
    only `dev`/`build`/`preview`/`type-check`), so verification is manual. That
    raises the value of a careful route-by-route check, and makes the combined
    Tailwind-major + library swap the riskiest part of this PRD.

## 9. Success Metrics

- `grep -ri primevue vue/src vue/*.json vue/*.ts` returns nothing (outside the
  lockfile history).
- `npm run build` and `npm run type-check` pass in the `ui` container.
- Every route (`/login`, `/maps`, `/contacts`, `/rulebook`, `/updates`,
  `/schedule`, `/faq`, `/sos`) renders with no console errors, including the
  overflow `MoreMenu`, the permission pre-prompts and the update prompt.
- Initial bundle (JS + CSS) is the same size or smaller than before, measured
  from the build output.
- The login flow still completes end to end against the BFF.
- `vue/package.json` shows `tailwindcss@^4`, no `autoprefixer`, and no
  `tailwind.config.js` remains (or only a minimal v4-relevant one).
- A visual before/after comparison of every route shows no unintended drift from
  the Tailwind v4 default changes.
- A developer or agent reading `.rules` and the new `vue3-pwa-layout` skill is told
  to use shadcn-vue, to prefer standard components over hand-rolled ones, and not
  to use PrimeVue.
- Generated primitives are usable from a new component with a single
  `@/components/ui/...` import.

## 10. Rollout / Task Breakdown

Single phase, no feature flag — this is a build-time change with no user-facing
switch. Sequence matters: **upgrade Tailwind first** (on its own commit, so the
codemod diff is reviewable in isolation and any visual drift is attributable),
then install the shadcn foundation, then remove PrimeVue, then document. Removing
PrimeVue last keeps the app in a working state throughout, and the v4 rewrite of
`main.css` already subsumes PrimeVue's cascade-layer workaround.

Recommended order: Tailwind v4 → shadcn foundation → primitives → PrimeVue
removal → optional `MoreMenu` refactor → docs/skills → verification.

Proposed tasks to create in `roadmap/tasks/open/`:

- [ ] Task: Upgrade Tailwind 3.4 → latest v4 in the `ui` container — run
      `npx @tailwindcss/upgrade` from a clean tree, switch to
      `@tailwindcss/vite`, drop `autoprefixer`/`postcss.config.js`, move config
      into CSS-first `@theme`, and review every codemod diff by hand.
- [ ] Task: Visual-regression pass after the Tailwind upgrade — walk every route on
      a phone viewport and fix drift from v4's changed defaults (borders, rings,
      shadows, `outline-hidden`).
- [ ] Task: Confirm and document the browser baseline implied by Tailwind v4
      (Safari 16.4+ / Chrome 111+) against the phones participants use.
- [ ] Task: Install the shadcn-vue foundation for Tailwind v4 in the `ui`
      container — `reka-ui`, `cva`, `clsx`, `tailwind-merge`, `tw-animate-css`,
      `cn()` in `@/helpers`, and a `components.json` whose aliases match repo
      conventions.
- [ ] Task: Add the shadcn CSS-variable design tokens to `main.css` via
      `@theme`/`@custom-variant dark` so the current look is preserved.
- [ ] Task: Generate the initial primitives into `src/components/ui/` —
      `button`, `input`, `sheet`/`drawer`, `dialog`, `card`, `separator` — with
      Lucide as the icon set; audit them for ≥44px touch targets.
- [ ] Task: Remove PrimeVue — uninstall the three packages and strip the wiring
      from `main.ts` and the resolver from `vite.config.ts`.
- [ ] Task: Decide and execute the `unplugin-vue-components` question — remove the
      plugin and add explicit imports for the app's own components (deleting
      `src/components.d.ts`), or keep it resolver-less with a comment saying why.
- [ ] Task: Refactor `MoreMenu.vue` onto the generated `sheet`/`drawer` primitive
      if it removes hand-rolled a11y/scroll-lock code without visual change.
- [ ] Task: Update `.rules` — shadcn-vue is the component library, standard
      components are preferred over hand-rolled ones, PrimeVue and PrimeIcons must
      not be used or reintroduced. *(Done ahead of approval at the user's request.)*
- [ ] Task: Rename `.agents/skills/vue3-spa-layout/` → `vue3-spa-layout-legacy/`
      and mark it as describing the older PrimeVue-based sibling repos, not this
      one. *(Done ahead of approval at the user's request.)*
- [ ] Task: Add the `.agents/skills/vue3-pwa-layout/` skill describing this repo's
      stack — Vue 3 + TS, Tailwind v4, shadcn-vue (standard-component-first),
      Pinia, PWA/service worker, Lucide. *(Done ahead of approval at the user's
      request.)*
- [ ] Task: Rebuild the `ui` image, run `build` + `type-check`, click through
      every route on a phone viewport, verify the service-worker update flow after
      the asset-hash churn, and record before/after bundle sizes in the task log.

## 11. Open Questions

**Resolved during implementation (2026-08-24):**

- **Browser baseline** — *accepted* (task 026). Safari 16.4+ / Chrome 111+. iOS
  shipped Web Push for home-screen apps in exactly 16.4, so the product already
  required this floor; Tailwind v4 costs zero reach.
- **Tailwind v4 config style** — *fully CSS-first*. `tailwind.config.js` deleted.
- **`@tailwindcss/vite` vs `@tailwindcss/postcss`** — *the Vite plugin*;
  `postcss.config.js` and `autoprefixer` deleted with it.
- **`unplugin-vue-components`** — *removed* (task 031). Every app component was
  already explicitly imported everywhere it was used, so this was a no-op rather
  than the risky sweep the PRD anticipated.
- **Sheet vs Drawer** — *Drawer, and `sheet` was dropped entirely* (task 029). It
  is the touch-first primitive, its default `swipeDirection` is a bottom sheet, and
  keeping both would leave "which one?" unanswered. `MoreMenu` now composes it
  (task 032).
- **Token palette** — *zinc*, not the app's slate: the shadcn registry no longer
  offers `slate` as a base colour. Harmless, because app components keep their
  explicit `slate-*` utilities. Unifying the two greys is a branding decision,
  left out of scope.
- **Dark mode** — *`.dark` token block kept* (unused, ~1 kB). Separately,
  `color-scheme` was corrected from `light dark` to `light`: the old value claimed
  dark support we do not have, so a phone in dark mode painted a dark canvas behind
  white surfaces.

**Discovered during implementation — not anticipated by this PRD:**

- **Node 22 was a hard prerequisite** (task 038). The shadcn-vue CLI crashes on
  Node 20 inside `undici`; the `ui` image was `node:20-alpine`, and Node 20 is
  EOL. Both `ui` stages were bumped.
- **Two Lucide packages** (task 037). `shadcn-vue init` installs `@lucide/vue`,
  while the app was on the deprecated `lucide-vue-next`. Migrated all 12 import
  sites and removed the old package rather than ship both.
- **The CLI adds a Google Fonts `@import`** for Inter. Removed — a blocking
  third-party font request is wrong for an app used on rural mobile data, and it
  would have been a visual change this PRD rules out.
- **The Tailwind codemod is not sufficient on its own.** It reported "0 files
  changed" for templates and was wrong: v4's `shadow-xs` is v3's `shadow-sm`, so
  `PermissionPrompt.vue` needed a manual fix. Conversely its border-colour compat
  shim was unnecessary here. A manual audit is mandatory, not optional.
- **Reka UI's Drawer is heavy** (~25 kB gzip). Importing it into the shell ate
  almost the entire PrimeVue saving, so `MoreMenu` is now lazily loaded.

**Still open:**

- **Toasts** — `ToastService` is gone and nothing replaced it. Drop toast
  capability entirely, or install `vue-sonner` (the shadcn-vue idiom) when PRD 002's
  tile-failure notice or PRD 003's upload errors need it? `UpdatePrompt.vue` is a
  hand-rolled banner and could set the pattern instead. **Deferred to the first
  feature that actually needs it.**
- **Other repos** — this rule change is scoped to `hej`, and the legacy skill was
  renamed rather than deleted precisely because sibling repos still match it. Do
  those repos move to shadcn + Tailwind 4 too, or does `hej` deliberately diverge
  as the modern reference?
- **Trimming unused primitives** — `dialog`, `card` and `separator` are generated
  but not yet used, costing ~3.8 kB gzip of CSS because Tailwind v4 scans source
  files rather than the import graph. Fine if PRD 002/003 consume them soon;
  revisit if they don't.
