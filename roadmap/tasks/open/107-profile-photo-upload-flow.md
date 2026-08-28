# 107 — Frontend: `ProfilePhoto.vue` upload flow

**Status:** open
**Priority:** medium
**Created:** 2026-08-28

## Description

Depends on tasks 105 and 106. (Task 102's consent blocker was cleared 2026-08-28.)

The portrait section of the profile page: current photo or a placeholder with a
clear call to action, tapping opens `PhotoCapture.vue`, upload shows **inline**
progress on the portrait (not a modal spinner), with retry and error states.

Composes the shadcn-vue `avatar` primitive from task 095; PRD 007 composes the
same primitive rather than reusing this component.

`fetchWrapper` may need extending for multipart.

Also feeds task 097: once a portrait exists, the user-menu avatar shows it
instead of initials.

## Acceptance Criteria

- [ ] Placeholder state with a call to action explaining *why* a portrait is
      wanted (night-time identification).
- [ ] Capture → upload → replace flow works end to end.
- [ ] Inline progress, retry on failure, error text in Danish.
- [ ] Meaningful `alt` text.
- [ ] `UserMenu.vue` avatar picks up the portrait.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
