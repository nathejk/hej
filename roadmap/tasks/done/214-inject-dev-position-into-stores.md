# 214 — Inject the dev position provider into the stores

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 3. Wire tasks 212/213 into `location.store` and `track.store` at the
seam they already have, and add the dev panel controls for it.

Both stores resolve their geolocation source through a small accessor
(`location.store.ts:150` and `track.store.ts:327` both check
`'geolocation' in navigator` before use). That accessor is the injection point: it
returns the dev provider when one is active, and the real `navigator.geolocation`
otherwise.

## Read location.store's WebKit comment first

`location.store.ts:84` records that WebKit answers `prompt` to
`navigator.permissions.query({name:'geolocation'})` even when permission is granted,
which once made the app conclude it had no permission on a cold start and refuse to
ask. The dev provider must not resurrect that: if a fake position is active, the
permission-state logic has to be consistent with it, or the app will show "location
off" while happily receiving positions — the exact contradiction PRD 014 forbids
(nothing may *look* like it works when it does not, and vice versa).

## Panel controls

Fake position on/off, coordinate preset, accuracy, failure mode, and playback
start/pause (task 213). Must be clearly marked as simulated, per task 208's rule.

## Acceptance Criteria

- [x] The stores take the dev provider through their existing accessor; no new
      branch inside their action bodies beyond that resolution
- [~] With a fake position active, the map shows a position and the accuracy circle — the
      provider feeds the store and the accuracy is asserted by test; the drawing is unverified
      in a browser
- [x] Permission state is consistent with the fake provider — no "location off" while
      positions are arriving; asserted by test
- [~] Playback moves the marker, and `track.store` logs points — movement is unit-tested and
      `track.store` now shares the same source; the recording is unverified in a browser
- [x] Failure modes are selectable and produce the codes `location.store` classifies
- [x] Panel labels the position as simulated
- [x] Real geolocation is restored the moment the override is off (per-call bridge, plus a
      permission re-sync)

## Depends on

- **Tasks 208, 212, 213.**

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 3.
- 2026-09-12 09:37 — `location.store` gains a registered provider consulted inside its existing
  `browserGeolocation()` accessor. No action bodies changed.
- 2026-09-12 09:38 — **Fixed a pre-existing inconsistency while here, deliberately.**
  `track.store.acquire()` read `navigator.geolocation` **directly** rather than going through
  `location.store`'s accessor. That is not merely untidy: the recorder and the map could resolve
  different sources, i.e. disagree about where the device is. It now calls a new exported
  `geolocationSource()`. Small product change, flagged here rather than buried.
- 2026-09-12 09:42 — The task's warning about permission state was justified, and needed real work.
  A laptop that has **denied** location for this origin would show "location off" while simulated
  positions arrived — and the store may not call the source at all in that state. Enabling the fake
  now folds `granted` in through the store's own `applyPermissionState`, so it claims exactly what a
  real grant claims by the same code path; disabling re-syncs from the browser. Asserted by test.
- 2026-09-12 09:43 — Panel controls added: fake on/off, accuracy, the five failure modes, and
  walk/pause/reset with a progress readout — all under an amber "position simulated" note.
- 2026-09-12 09:43 — ✅ Suite 468/468 across 39 files; `vue-tsc` clean.
