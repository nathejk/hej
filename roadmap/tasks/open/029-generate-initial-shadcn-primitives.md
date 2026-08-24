# 029 — Generate the initial shadcn-vue primitives

**Status:** open
**Priority:** high
**Created:** 2026-08-24
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `button`, `input`, `sheet`/`drawer`, `dialog`, `card`, `separator` exist
      under `src/components/ui/` and are committed.
- [ ] The Sheet-vs-Drawer decision is made and recorded in the progress log.
- [ ] Each generated component is audited for ≥44px touch targets; deviations
      from upstream are commented in the source.
- [ ] Components use Lucide icons only — no other icon set is pulled in.
- [ ] A primitive can be used from a new component with a single
      `@/components/ui/...` import.
- [ ] `npm run type-check` and `npm run build` pass in the `ui` container.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
