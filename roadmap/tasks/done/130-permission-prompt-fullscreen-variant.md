# 130 — PermissionPrompt: full-screen onboarding variant

**Status:** done
**Priority:** medium
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

PRD 005 §7. `components/PermissionPrompt.vue` today renders a compact card: an optional
Lucide icon, a title, a message, an optional "læs mere" link, an accept button and an "Ikke
nu" dismiss. It is designed to sit *inside* a page, next to whatever context made it
relevant.

Onboarding needs the same content as a **full screen**: one explanation per step, filling the
`/welcome` shell, read before any native dialog appears. Rather than a second component
duplicating the copy, the props, the "soft pre-prompt so a decline doesn't burn the browser
permission" rationale and the accept/dismiss contract, add a **`variant` prop** so one
component serves both presentations.

**PRD 005 owns this component's API.** PRD 002 (the map's repair affordance) and PRD 003
(the profile page's status rows) are *consumers* of that API, not co-owners — so their call
sites must keep working unchanged. That is the binding constraint on this task: the compact
variant is the default, and no existing call site may need editing to keep the appearance it
has today.

Today's surface is `{ title, message, cta, icon?, moreTo?, moreLabel? }` with `accept` /
`dismiss` emits. Both optional props have history worth respecting:

- `moreTo` / `moreLabel` arrived with **task 085**, because the location prompt asks for
  something bigger than two lines can describe — the route is recorded and sent to the
  organizers — so it needs somewhere to point. `moreLabel` defaults to `'Læs mere'`.
- **Task 101** shipped the blocked-permission guidance (`config/permissions.ts`) for the case
  where the permission is already denied at OS level and no prompt will ever appear again.

This task **extends** that work rather than replacing it (PRD 005 §7, corrected 2026-08-30).
Both must be reachable from the full-screen variant: an onboarding explanation is exactly a
place where "læs mere" belongs, and a step reached with the permission already denied needs
task 101's settings guidance rather than an accept button that does nothing.

Design notes:

- Default the variant to the existing compact rendering, so omitting the prop is the current
  behaviour byte-for-byte.
- Keep it a presentation switch. If the variant starts changing *what* is asked or *what
  happens on accept*, the split belongs in the caller (task 131's steps), not in here.
- The full-screen variant lives inside the `/welcome` shell, which already provides safe-area
  padding and the progress indicator — so it should not add its own page chrome or a second
  layer of `--sat`/`--sab`.
- "Ikke nu" is the right dismiss wording in a page; in onboarding the same emit means "spring
  over". Make the dismiss label overridable rather than branching on variant inside the
  template.
- Headline in the full-screen variant is a page-level heading and uses `font-nathejk` per
  `.rules`; icons stay Lucide. Copy in Danish.
- Accessibility (PRD 005 §6): operable without gestures, and the explanation readable as
  text.

## Acceptance Criteria

- [x] `PermissionPrompt.vue` takes a `variant` prop supporting the existing compact card and
      a full-screen onboarding presentation
- [x] The compact variant is the **default**; every shipped call site (PRD 002 map, PRD 003
      profile rows) renders exactly as before with no edits
- [x] `title` / `message` / `cta` / `icon` / `moreTo` / `moreLabel` and the `accept` /
      `dismiss` emits are unchanged in meaning, with `moreLabel` still defaulting to
      `'Læs mere'`
- [x] `moreTo` / `moreLabel` (task 085) work in the full-screen variant
- [x] Task 101's blocked-permission guidance is reachable from the full-screen variant, so an
      already-denied permission shows settings guidance instead of a useless accept button
- [x] The dismiss label is overridable, so onboarding can say "spring over" without a
      variant branch in the template
- [x] The full-screen variant adds no page chrome or duplicate safe-area padding of its own
- [x] Full-screen headline uses `font-nathejk`; icons Lucide; copy Danish
- [x] Operable without gestures; the explanation is text, not an image
- [x] `vue-tsc` and `npm run build` clean; both variants checked visually — *the visual check is
      deferred to task 139's device pass (no browser here); the compact path is unchanged markup,
      so the risk is confined to the new variant.*

## Depends on

- Nothing blocking. **Task 085** (the `moreTo`/`moreLabel` props and location copy) and
  **task 101** (blocked-permission guidance) are shipped and must be preserved.
- **Task 131** is the first consumer of the new variant.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — Picked up.
- 2026-08-30 — **`variant: 'compact' | 'page'` added, defaulting to `compact`.** The compact
  branch is the previous template unchanged, so the two shipped call sites — `MapsView.vue:168`
  and `UpdatesView.vue:42` — needed no edits and render as before.

  Three decisions:

  - **`dismissLabel` is a prop, not a `variant` branch.** "Ikke nu" is right in a page full of
    other content; "spring over" is right in a linear flow. That is a copy decision belonging to
    the caller, and branching on the variant inside the template would have hidden it here — where
    the next person to add a variant would have to discover it.
  - **Blocked state is `blocked` + `blockedGuidance` props rather than an import of
    `config/permissions.ts`.** Keeps this component free of the permission domain: it still only
    renders what it is told, so the same component can front a permission it knows nothing about.
    Task 131's steps pass task 101's `blockedGuidance(...)` through. When blocked, the accept
    button is **replaced** rather than disabled — a disabled "Tillad" still reads as "the app is
    broken", whereas the settings sentence is the only action that can actually work.
  - **The page variant deliberately has no `--sat`/`--sab` padding and no outer chrome.** It
    renders inside the `/welcome` shell (task 124), which owns both; a second inset would show up
    as a stripe of dead space under the notch.
- 2026-08-30 — ✅ `vue-tsc` and `npm run build` clean. Moving to done; the visual pass on the new
  variant rides along with task 139.
