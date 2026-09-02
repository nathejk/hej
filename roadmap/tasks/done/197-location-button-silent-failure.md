# 197 — "Slå placering til" can do nothing at all

**Status:** done
**Priority:** high
**Created:** 2026-09-02
**Picked up by:** agent session (Zed)
**Started:** 2026-09-02
**Completed:** 2026-09-02

## Description

Found on an **iPad, iPadOS, installed to the home screen** (maintainer's device run, 2026-09-02, task
139's matrix): on the map, tapping **"Slå placering til"** produced *nothing*. No iOS dialog, no error,
no change. The profile page continued to report `Placering — Fra`.

The underlying WebKit behaviour is not yet known. **The bug being fixed here is that the app cannot
say.** Every failure path is silent, and there are three of them:

1. **The call hangs.** `getCurrentPosition`'s `timeout` option bounds *acquiring a fix*, not waiting for
   the user to answer the permission dialog. If the dialog never appears — or appears and is never
   answered — neither callback ever fires, the promise never settles, and the UI stays exactly as it was
   for ever. There is no wall-clock guard anywhere.
2. **An error that is not `PERMISSION_DENIED`.** `location.store.request()` only changes state for that
   one code. `POSITION_UNAVAILABLE` — which is what iOS reports when **Location Services is off for the
   whole device** — and `TIMEOUT` leave `permission` at `'prompt'`, so the prompt card just sits there
   looking untouched.
3. **No pending state.** `PermissionPrompt` has no busy prop. A tap emits `accept` and the button looks
   identical afterwards, so even a *working* request that takes eight seconds is indistinguishable from
   a dead button.

Two smaller faults sit alongside:

- The one error line that does exist says "Kunne ikke finde din placering" for every cause, so "you
  turned this off in Settings", "the whole device has Location Services off" and "we could not get a fix
  in time" are one message. Those need different actions from the user.
- `blockedGuidance('location')` exists (task 101) and tells the user where the iOS setting is. Nothing
  on the map reaches it, because that path is only wired for `denied`, which cause 2 never sets.

## Why this matters more than a normal UI bug

Location is what the position track is built on (PRD 002 §11.1), and the track is the one thing on the
device that cannot be re-fetched. A participant who taps the button, sees nothing, shrugs and walks into
the forest has silently opted out of the whole feature — and we would have no record of why.

## Acceptance Criteria

- [x] A tap on "Slå placering til" **always** produces a visible change within a moment: the button
      becomes "Venter på telefonen…", then success, denial, or a named failure.
- [x] A wall-clock guard (`GEO_STUCK_MS`, 25 s) means a hung `getCurrentPosition` cannot leave the UI in
      limbo — the app's own clock, not the API's `timeout`.
- [x] The error codes are distinguished, in Danish, with the right advice for each. `POSITION_UNAVAILABLE`
      now points at **Stedtjenester**, not at this app's own permission.
- [x] `blockedGuidance('location')` is reachable from the map when the permission is blocked.
- [x] The failure is recorded as a `geoerror` event in the track's diagnostic log, with cause and the
      browser's own message.
- [x] Tests cover each error code and the hang, with the geolocation API injected.

## Progress Log

- 2026-09-02 — Task created from the iPad run. Note the same run **passed** the two things task 139
  expected to be riskiest: iPadOS classified as mobile, and standalone was detected once installed.

- 2026-09-02 — **Diagnosed as four silent paths, not one bug.** The WebKit behaviour on that iPad is still
  unknown, and fixing it did not require knowing: whatever happened, the app had no way to say. Repaired
  all four —

  1. **The hang.** `getCurrentPosition`'s `timeout` option bounds *acquiring a fix*; it does not bound
     waiting for the permission dialog to be answered. If neither callback fires, the promise never
     settles. That is almost certainly what the iPad did, and it made the promise, the prompt and the
     button wait for ever. Now guarded by our own 25 s clock — long enough for a human to read an iOS
     dialog, short enough that a button cannot look dead.
  2. **Codes other than `PERMISSION_DENIED` changed nothing.** `POSITION_UNAVAILABLE` — what iOS reports
     when **Location Services is off for the whole device** — left `permission` at `'prompt'`, so the card
     sat there looking untouched. It now has its own cause and its own sentence, pointing at
     Stedtjenester rather than at this app: sending someone to the wrong Settings screen is worse than
     saying nothing.
  3. **No pending state.** `PermissionPrompt` had no `busy` prop, so even a *working* eight-second request
     was indistinguishable from a dead button. This is the part that made the bug cost a user: they tapped,
     saw nothing, and stopped.
  4. **A blocked permission hid the card entirely**, which meant task 101's settings guidance — the one
     thing that helps a user who has already refused — was unreachable from the map.

- 2026-09-02 — **`classifyGeoError` reads the numeric codes, not `err.PERMISSION_DENIED`.** Those constants
  live on the error instance, so a stubbed error or a browser returning a plain object does not carry them
  — and the old code compared against `err.PERMISSION_DENIED`, which is `undefined` on such an object,
  making `undefined === undefined` true and classifying *every* malformed error as a denial. The values are
  fixed by the spec, so using 1/2/3 is both testable and more robust.

- 2026-09-02 — An unrecognised code is deliberately `unavailable`, never `denied`: guessing "denied" tells a
  user they refused something they never did, and sends them to a Settings screen to undo it.

- 2026-09-02 — Geolocation is now injected into the store (`geo`), following `helpers/platform.ts`: the
  cases worth testing — a call that never answers, a device with location switched off — cannot be produced
  on the machine running the tests.

- 2026-09-02 — ✅ All criteria complete. 14 new tests; suite 371 across 31 files; `type-check` and `build`
  clean. **The original symptom is not proven fixed** — nobody has re-run it on the iPad. What is fixed is
  that the next attempt will *say* what happened, in Danish, and log the cause for the run after that.
