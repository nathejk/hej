# 034 — Rename the frontend skill to vue3-spa-layout-legacy

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

`.agents/skills/vue3-spa-layout/` instructed agents to use PrimeVue unstyled +
the Lara preset, PrimeIcons and `src/presets/lara/`, and listed "Don't override
PrimeVue look in components" among its don'ts. After task 033 that directly
contradicted `.rules` — a live trap for the next agent session.

Renamed rather than deleted: sibling Nathejk repos still run that stack, and the
routing, Pinia, `fetchWrapper` and container conventions in it are still shared.

Completed ahead of PRD approval at the user's explicit request.

## Acceptance Criteria

- [x] `.agents/skills/vue3-spa-layout/` is renamed to
      `.agents/skills/vue3-spa-layout-legacy/` (the empty old directory is gone).
- [x] The skill's `name:` frontmatter matches the new directory name.
- [x] The `description:` marks it LEGACY, describes it as covering the older
      PrimeVue-based sibling repos, and says explicitly **not** to apply it to
      `hej`, pointing to `vue3-pwa-layout` instead.
- [x] A callout at the top of the body repeats that warning and references
      `.rules` / PRD 004.
- [x] The reason for keeping rather than deleting it is stated in the file.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004 (retroactively — work was done
  before the PRD was approved, at the user's request).
- 2026-08-24 00:00 — Moved `SKILL.md` to the new directory, rewrote the
  frontmatter (name + LEGACY description with the do-not-apply-here warning) and
  added a blockquote callout above the heading. Retitled the document "Vue 3 SPA
  Layout (legacy, PrimeVue)".
- 2026-08-24 00:00 — Deleted the empty `vue3-spa-layout/` directory left behind by
  the move.
- 2026-08-24 00:00 — Left the historical references to the old skill name in
  `roadmap/prd/done/001-*.md` and `roadmap/tasks/done/001-*.md` untouched: those
  are records of what was decided at the time, and the task board forbids editing
  past log entries.
- 2026-08-24 00:00 — Completed.
