# 101 — Frontend: blocked-permission guidance copy in one place

**Status:** done
**Priority:** low
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

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

- [x] One module (`src/config/permissions.ts`) exporting guidance per
      capability × platform, in Danish.
- [x] Platform detection is contained in that module; components never sniff the
      UA themselves.
- [x] Task 099's rows consume it; no duplicated strings. (Lands in the very next
      commit — the two tasks share the same rows.)
- [ ] Unit test for the capability × platform matrix — **not done: no unit-test
      runner in this repo** (see task 098). `Record<Capability, Record<Platform,
      string>>` at least makes a missing combination a type error rather than an
      `undefined` rendered to the user.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — `detectPlatform()` branches on **platform, not browser**: on iOS
  every browser is WebKit and the Settings path is identical, so a "Safari vs
  Chrome" split would have produced two identical answers and one wrong one. It
  also handles iPadOS reporting a desktop-macOS UA (`Macintosh` +
  `maxTouchPoints > 1`), which would otherwise have fallen through to the generic
  text.
- 2026-08-28 — `blockedGuidance(capability, platform?)` takes the platform as an
  optional argument so a caller or future test can request specific text without
  faking a user agent.
- 2026-08-28 — Copy is phrased as "where to go" rather than "you denied this", and
  the notifications entry names *Hej Nathejk* on iOS because a home-screen web app
  gets its own entry under Notifications there — pointing at Safari would send the
  user to the wrong screen.
- 2026-08-28 — ✅ Module in place and consumed by the profile rows;
  `npm run type-check` and `npm run build` clean. Moving to done.
