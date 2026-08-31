# 172 — Offline test protocol on real devices

**Status:** open
**Priority:** medium
**Created:** 2026-08-31

## Description

PRD 007's success metrics require verification **by test on a real device, not by
assumption** — the pane's whole premise is behaviour under conditions a desktop browser
does not reproduce.

Two halves, and they must not be conflated:

- The **directory works with the radio off** — list, groups, search, favourites,
  profiles, portraits.
- The **patrol lookup fails clearly** with the radio off — "kræver forbindelse" and a
  pointer to the radio, never an empty or stale patrol.

Cover both iOS/iPadOS Safari 16.4+ (home-screen install) and Chrome 111+ on Android, per
the `.rules` baseline. iOS specifically, because that is where cache eviction bites and
where the install path differs.

Also worth exercising in the same pass: launch after several days unused (eviction), and
airplane mode toggled mid-session (reconnect triggering a version check, task 162).

## Acceptance Criteria

- [ ] A written protocol in this file: device, OS version, install method, steps.
- [ ] Directory verified fully usable offline on both platforms, installed to home
      screen.
- [ ] Patrol lookup verified to fail clearly offline on both platforms.
- [ ] Eviction scenario exercised: cache cleared → pane degrades to names-only, then
      recovers on reconnect.
- [ ] Reconnect triggers a freshness check without a manual action.
- [ ] Results recorded in the progress log, including anything that surprised us.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §9.
