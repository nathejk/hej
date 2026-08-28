# 108 — Device testing pass: profile page on real phones

**Status:** open
**Priority:** medium
**Created:** 2026-08-28

## Description

Blocked by task 102; runs after 107.

Camera, permissions and orientation cannot be verified in a desktop browser. Test
on **iOS Safari as an installed standalone PWA** and on **Android Chrome**
(baseline per `.rules`: iOS 16.4+ / Chrome 111+).

Cover: portrait capture in both orientations, front/rear flip, denying camera and
recovering via the guidance copy, the permission rows reflecting a change made in
system/browser settings after returning to the app, and the user menu's tap
target and dismissal behaviour on touch.

## Acceptance Criteria

- [ ] Capture works on both platforms, upload persists across a reload and on a
      second device.
- [ ] Permission rows match real device state after changing it in settings.
- [ ] A previously denied notification permission can be recovered following only
      the on-page guidance.
- [ ] Findings logged in this task; regressions filed as their own tasks.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
