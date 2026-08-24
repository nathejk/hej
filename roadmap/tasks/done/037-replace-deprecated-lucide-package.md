# 037 — Replace the deprecated lucide-vue-next with @lucide/vue

**Status:** done
**Priority:** low
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

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

- [x] `lucide-vue-next` is removed; `@lucide/vue` is installed at a current
      version.
- [x] No import of `lucide-vue-next` remains anywhere in `vue/src`.
- [x] Generated `src/components/ui/` components import from the same package.
- [x] `.rules` and the `vue3-pwa-layout` skill name the new package.
- [x] `npm ci` no longer warns about a deprecated Lucide package.
- [x] `type-check` and `build` pass; icons render unchanged on every route.

## Progress Log

- 2026-08-24 00:00 — Task created. Found via an `npm ci` deprecation warning while
  working task 024; recorded rather than fixed inline to keep that diff clean.
- 2026-08-24 01:58 — Pulled forward from "later" to "now": `shadcn-vue init`
  installed `@lucide/vue` for its generated components, so the repo briefly shipped
  **both** Lucide packages with app code on the deprecated one. Leaving that was
  worse than either end state.
- 2026-08-24 02:00 — Checked before rewriting rather than after: verified all 15
  icons the app uses (`LogOut`, `Map`, `Users`, `BookOpen`, `Megaphone`,
  `CalendarDays`, `Siren`, `HelpCircle`, `RefreshCw`, `Menu`, `KeyRound`, `Phone`,
  `ArrowLeft`, `Bell`, `MapPin`) exist in `@lucide/vue` — none missing, 6099 exports
  available. So this is a pure import-path change with no icon renames.
- 2026-08-24 02:02 — Rewrote all 12 import sites, uninstalled `lucide-vue-next`.
- 2026-08-24 02:03 — ✅ type-check + build clean; no deprecation warning left.
- 2026-08-24 02:04 — Docs: `.rules` and the `vue3-pwa-layout` skill updated to name
  `@lucide/vue`. Completed.
