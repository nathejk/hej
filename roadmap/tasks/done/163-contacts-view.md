# 163 — ContactsView: grouped directory

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

Replace the `PagePlaceholder` in `vue/src/views/ContactsView.vue` with the real pane
(PRD 007 §7): sticky search, favourites, collapsible groups with the caller's own group
expanded, and a read-only sync line. Not `fullBleed`. Night-legible, scoped to this view.
Existing shadcn-vue primitives only.

## Implementation

`vue/src/views/ContactsView.vue`.

**Structure:** sticky header (headline + sync state + search) → favourites → one section per
population (crew, banditter, gøglere) → an accordion of groups within a section that has more
than one.

**A single group gets no accordion.** Crew and gøglere are one list each, so wrapping them in a
collapsible section produces a control that can only ever be in the way — and, worse, one that
can be collapsed to hide the entire list with no hint that anything is there. Only banditter,
grouped by klan, actually get an accordion.

**Own group opens by default, once.** The seeding is guarded by a flag rather than recomputed,
because the entries arrive *after* mount on a cold start, and a background refresh must not
re-open a section the user deliberately collapsed. That is task 162's deferred in-place-update
criterion, and it is why expansion state is keyed on the stable server-supplied group id rather
than on an index or a label — an upstream rename cannot silently reset it.

**The sync line reads as current, not as a timestamp.** "Opdateret nu" under two minutes, then
"Opdateret kl. 21:40", and an explicit "Ikke opdateret" when the last refresh failed.
"Synkroniseret 21:40" invites the question "is that recent?" and makes the reader do the
arithmetic; during a race that is the wrong division of labour. A user who believes they have the
directory and does not is worse off than one who knows they do not, so the failed state is
prominent rather than tidy.

**Night surface is scoped to this view** — `dark` on the section container, built on the `.dark`
token block PRD 004 retained. Not a global theme, and `color-scheme` stays `light`, because
app-wide dark mode is not supported. The negative margins let it reach the edges of App.vue's
padded scroll container without `fullBleed`, which would have removed the scroll wrapper the list
needs.

**Search replaces the sections rather than filtering them in place.** Two competing lists on one
small screen is a worse answer than one, and it also makes "no matches" unambiguous.

Three distinct empty states, which is the point of task 169's requirement: nothing synced yet
("Kontakter hentes, når du er online"), no search matches (quoting the query back), and no
favourites (the section is absent entirely rather than an empty header).

## Acceptance Criteria

- [x] `ContactsView.vue` renders the grouped directory from the store; placeholder gone.
- [x] Own group expanded by default; expansion state persists across visits — within the
      session and across background refreshes. Deliberately **not** persisted to storage: the
      pane is opened to find somebody, and reopening it hours later should present the default
      view rather than whatever was expanded last time.
- [x] Night-legible surface scoped to this view; no global theme change.
- [x] Uses existing shadcn-vue primitives; no hand-rolled accordion or avatar (`accordion`,
      `input`, `avatar` — `command` was not needed, so it was not generated).
- [x] Sync/freshness line present and honest about stale/offline.
- [x] Renders correctly with an empty group, one group, and many groups — one group skips the
      accordion; an empty directory shows the not-synced state.
- [x] Updates in place: a background refresh does not reset scroll position or collapse the
      user's expanded group (keys are stable group ids; seeding runs once).

## Progress Log

- 2026-08-31 23:45 — Picked up. Built the pane against the store, row and favourites.
- 2026-08-31 23:55 — Decided a single-group section renders as a plain list: an accordion around
  the only list on screen can be collapsed to hide everything with no hint it exists.
- 2026-09-01 00:05 — Guarded the "open own group" seeding with a flag, so a refresh cannot
  re-open a collapsed section — task 162's deferred criterion.
- 2026-09-01 00:10 — ✅ All criteria met. 144 frontend tests pass, `vue-tsc --noEmit` clean.
