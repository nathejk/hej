# 214 — Inject the dev position provider into the stores

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] The stores take the dev provider through their existing accessor; no new
      branch inside their action bodies beyond that resolution
- [ ] With a fake position active, the map shows a position and the accuracy circle
- [ ] Permission state is consistent with the fake provider — no "location off" while
      positions are arriving
- [ ] Playback moves the marker, and `track.store` logs points
- [ ] Failure modes reach the map's "location off" state and `track.store`'s failure
      dedup path
- [ ] Panel labels the position as simulated
- [ ] Real geolocation is restored the moment the override is off

## Depends on

- **Tasks 208, 212, 213.**

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 3.
