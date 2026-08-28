# 101 — Frontend: blocked-permission guidance copy in one place

**Status:** open
**Priority:** low
**Created:** 2026-08-28

## Description

When a permission is `denied`, the browser will not prompt again — an enable
button is a dead end. PRD 003 requires platform-appropriate guidance instead, and
requires the copy to live in **one** place so `PermissionPrompt.vue` (PRD 005),
the profile rows (task 099) and the map's location-off state (PRD 002) cannot
drift into three different sets of instructions.

iOS Safari (incl. installed PWA) and Android Chrome are the platforms that
matter; `.rules` fixes the baseline at iOS 16.4+ / Chrome 111+, so no legacy
fallbacks.

## Acceptance Criteria

- [ ] One module (e.g. `src/config/permissions.ts` or
      `src/helpers/permissionGuidance.ts`) exporting guidance per
      capability × platform, in Danish.
- [ ] Platform detection is contained in that module; components never sniff the
      UA themselves.
- [ ] Task 099's rows consume it; no duplicated strings.
- [ ] Unit test for the capability × platform matrix, including an unknown
      platform falling back to generic wording.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
