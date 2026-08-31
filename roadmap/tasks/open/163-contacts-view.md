# 163 — ContactsView: grouped directory

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

Replace the `PagePlaceholder` in `vue/src/views/ContactsView.vue` with the real pane
(PRD 007 §7). The `contacts` destination already exists in `navigation.ts` — no new
route for the list.

Layout, top to bottom:

1. sticky search field (task 165);
2. favourites section (task 166);
3. collapsible groups from the server-supplied grouping (task 153), the caller's own
   group expanded, expansion state remembered per user;
4. read-only sync/freshness line.

Notes:

- **Not `fullBleed`.** The list scrolls, and `fullBleed` removes the `overflow-y-auto`
  wrapper in `App.vue`.
- Use the existing shadcn-vue primitives — `accordion` for groups, `input` for search,
  `avatar` for thumbnails. All already generated in `vue/src/components/ui/`; only
  `command` would need generating and only if search wants the palette treatment.
- Night-legible: dark, high-contrast surface built on the `.dark` token block PRD 004
  retained. **Scoped to this view** — not a global theme, and `color-scheme` stays
  `light` (`main.css:62`), because app-wide dark mode is not supported.
- The sync line should read as *current* during the race ("Opdateret nu") rather than a
  timestamp the user must interpret, with an explicit stale/offline state.
- `font-nathejk` on the page headline only; never set a font family in a component.

## Acceptance Criteria

- [ ] `ContactsView.vue` renders the grouped directory from the store; placeholder gone.
- [ ] Own group expanded by default; expansion state persists across visits.
- [ ] Night-legible surface scoped to this view; no global theme change.
- [ ] Uses existing shadcn-vue primitives; no hand-rolled accordion or avatar.
- [ ] Sync/freshness line present and honest about stale/offline.
- [ ] Renders correctly with an empty group, one group, and many groups.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §7.
