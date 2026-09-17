# 309 — GlimtModerationView — Team-section moderation UI

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §7. Not a separate tool — Team-section members are in the app, on their phones, at the
event. `/glimt/moderation`, reusing the feed components with three differences: every scope is
included, each card shows its audience and report count, and the overflow carries *Skjul* /
*Vis igen*. Default sort puts reported-and-not-yet-reviewed first.

Visible only when the Team section is assigned — the route is gated, and the nav entry does
not appear otherwise. The client gate is convenience; the server check (task 300) is the
actual control.

## Acceptance Criteria

- [ ] `/glimt/moderation` route, gated on the caller being in the Team section
- [ ] `GlimtModerationView.vue` reuses `GlimtCard` rather than duplicating it
- [ ] Each card shows audience and report count
- [ ] Hide / unhide actions call the task 308 endpoints and update optimistically
- [ ] Reported-first ordering
- [ ] Not reachable or visible for a non-Team member
- [ ] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
