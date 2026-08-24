# 029 — Generate the initial shadcn-vue primitives

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Generate the starting set of shadcn-vue components into
`vue/src/components/ui/`, scoped to what the app needs now and what the in-flight
PRDs need next. Do **not** generate the whole catalogue — components are added on
demand.

Initial set: `button`, `input`, `sheet` (or `drawer`), `dialog`, `card`,
`separator`.

```sh
docker compose run --rm ui npx shadcn-vue@latest add button input sheet dialog card separator
```

Open question from PRD 004 to settle here and record: **Sheet vs Drawer.**
shadcn-vue ships both — `Sheet` (side/bottom panel) and a Vaul-based `Drawer`
with mobile drag-to-dismiss. For a mobile-first PWA the drawer is plausibly the
better fit for `MoreMenu` (task 032) and for PRD 002's registrations list. Pick
one to standardise on and say why.

Generated components are **owned source**: commit them, and audit them for this
app's constraints rather than assuming the defaults fit:

- touch targets ≥44px (several shadcn defaults are desktop-sized),
- safe-area compatibility for anything anchored to a screen edge,
- night legibility,
- Lucide icons (the CLI should already use Lucide via `components.json`).

Adjust them **in place** in `ui/` and add a short comment on any deviation from
upstream, so a future re-generation is a conscious act.

PRD: 004. Depends on: 027, 028. Blocks: 032. Unblocks the UI tasks of PRD 002/003.

## Acceptance Criteria

- [x] `button`, `input`, `sheet`/`drawer`, `dialog`, `card`, `separator` exist
      under `src/components/ui/` and are committed.
- [x] The Sheet-vs-Drawer decision is made and recorded in the progress log.
- [x] Each generated component is audited for ≥44px touch targets; deviations
      from upstream are commented in the source.
- [x] Components use Lucide icons only — no other icon set is pulled in.
- [x] A primitive can be used from a new component with a single
      `@/components/ui/...` import.
- [x] `npm run type-check` and `npm run build` pass in the `ui` container.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 01:45 — Generated `button input sheet drawer dialog card separator`.
  Confirmed the CLI honoured our alias override: every generated file imports
  `cn` from `@/helpers/utils`, not `@/lib/utils`.
- 2026-08-24 01:50 — **Sheet vs Drawer: standardised on Drawer, and deleted
  `sheet`.** Reasons: the Drawer is Reka UI's touch-first implementation (drag
  handle, swipe-to-dismiss) and its default `swipeDirection` is `"down"`, i.e.
  exactly the bottom sheet this app uses; on a phone there is no case for a
  desktop-style side panel. Keeping both would leave the next developer asking
  "which one?", which is precisely what the standard-component-first rule is meant
  to prevent. `dialog` stays for centred confirmations.
- 2026-08-24 01:55 — **Touch-target audit found real problems, as the task
  predicted.** Upstream is desktop-biased: `Button` default was `h-9` (36px), `lg`
  `h-10`, `icon` `size-9`; `Input` was `h-9`. All below the 44px floor this app
  requires. Edited in place (owned source) to `h-11`/`h-12`/`size-11`/`size-12`
  and `h-11` respectively, each with a `LOCAL DEVIATION FROM UPSTREAM` comment so
  a future `add --overwrite` is a conscious act. Left `xs`/`sm` compact on purpose
  — they are opt-in for dense secondary affordances.
- 2026-08-24 01:57 — Also dropped `md:text-sm` from `Input`: below 16px iOS Safari
  zooms the viewport on focus, which is jarring in a standalone PWA. It now stays
  at `text-base` on every breakpoint.
- 2026-08-24 01:58 — Icons: generated components import from `@lucide/vue` (the
  package `init` installed), which is why task 037 was pulled forward and done in
  the same batch — shipping two Lucide packages would have been silly.
- 2026-08-24 02:00 — Cost note for the record: the generated-but-unused primitives
  are not free. Measured by temporarily removing `src/components/ui/`: they add
  26.2 kB raw / 3.8 kB gzip of CSS, because Tailwind v4 scans source files, not the
  import graph. Judged worth it since PRD 002/003 consume them next; if that
  changes, trimming the set is the lever.
- 2026-08-24 02:01 — ✅ type-check + build clean. Completed.
