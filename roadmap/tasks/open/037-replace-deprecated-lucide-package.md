# 037 — Replace the deprecated lucide-vue-next with @lucide/vue

**Status:** open
**Priority:** low
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

## Description

`npm ci` warns:

```
npm warn deprecated lucide-vue-next@0.454.0: Package deprecated. Please use @lucide/vue instead.
```

Found incidentally while doing task 024. Not in scope for PRD 004, but it should
not sit unrecorded: Lucide is the repo's mandated icon set (`.rules`), it is
imported across the app (`config/navigation.ts`, `BottomNav.vue`,
`PagePlaceholder.vue`, `UpdatePrompt.vue`, `MapsView.vue`, `UpdatesView.vue`,
`LoginView.vue`, …), and the package we depend on is both **deprecated** and
pinned well behind current.

Work:

- Swap `lucide-vue-next` → `@lucide/vue` (check the migration notes for renamed
  icons and any import-path change).
- Update every `from 'lucide-vue-next'` import.
- Update the icon references in `.rules` and
  `.agents/skills/vue3-pwa-layout/SKILL.md`, both of which currently name
  `lucide-vue-next`.
- Verify icons still render at the right size/stroke in the bottom nav, the
  permission prompts and the login screen.

Best done **after** PRD 004's tasks land, so it does not tangle with the
Tailwind/shadcn diffs. Note shadcn-vue components also import Lucide icons, so
task 029's generated primitives must use the same package.

## Acceptance Criteria

- [ ] `lucide-vue-next` is removed; `@lucide/vue` is installed at a current
      version.
- [ ] No import of `lucide-vue-next` remains anywhere in `vue/src`.
- [ ] Generated `src/components/ui/` components import from the same package.
- [ ] `.rules` and the `vue3-pwa-layout` skill name the new package.
- [ ] `npm ci` no longer warns about a deprecated Lucide package.
- [ ] `type-check` and `build` pass; icons render unchanged on every route.

## Progress Log

- 2026-08-24 00:00 — Task created. Found via an `npm ci` deprecation warning while
  working task 024; recorded rather than fixed inline to keep that diff clean.
