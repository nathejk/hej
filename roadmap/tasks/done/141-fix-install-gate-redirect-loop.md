# 141 — Fix: the install gate could redirect forever

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

Reported by the maintainer, 2026-08-30: the desktop placeholder looked fine on a laptop, but
loading `/` **on mobile never finished — the page just hung**.

Not a hang in the usual sense. The router guard (task 137) contained an infinite redirect, and
`vue-router` responds to that by aborting the navigation: no route component mounts, nothing
renders, and the tab sits there loading forever with nothing in the UI to say why.

## The cycle

Two individually sensible rules that pointed at each other's destination:

```
maps → (auth: not signed in)            → welcome
welcome → (gate: onboarding complete)   → maps
maps → …
```

The gate's rule was `complete && to.name === 'welcome' → maps`, meaning "you have finished
onboarding, you do not belong on the welcome flow". Reasonable in isolation, and fatal in
combination with the auth fallback, which sends an unauthenticated visitor *to* `/welcome`
because that is where login now lives (task 126).

**The state it needs is completely ordinary**, which is why this was not exotic:
`hej.onboarding.complete` is a per-device flag with no expiry, while the session cookie lasts
7 days. So: complete onboarding once, come back a week later, and the app is bricked — no
error, no wall, no login screen, just a page that never loads. That is the worst possible
failure for this app, since the affected user is the returning participant during an event.

**Why the laptop looked fine:** the desktop branch returns before any of this and leaves for
the static placeholder, so a laptop never reaches the auth step at all. The one platform that
exercises the cycle is the one the app is for.

## The fix

Remove the rule. Leaving `/welcome` is `WelcomeView`'s job, and it is the right owner: it
redirects when there is no unsettled step left **and** the user is authenticated — so it
cannot fire in the state that caused the loop. The gate does not need to know about the
session, which is the entire reason it runs before auth (PRD 005 §11).

## Also: make this class of bug testable

The gates lived inside `router/index.ts`, which calls `createWebHistory()` at import time and
therefore needs a `window` — so they could not be unit-tested at all, which is how a
reachable infinite loop shipped in the first place. Two changes:

- Gates moved to `router/gates.ts`, importable without a DOM.
- The desktop branch **returns a decision (`LEAVE_APP`) instead of calling
  `window.location.replace`**. A function that navigates cannot be tested; a function that
  returns "leave" can. The guard performs it.

The new test drives the gate *and* the auth fallback to a fixpoint, and asserts termination
across every combination of device class, standalone, escape-hatch override, onboarding
completion and session state — 120 paths. It is written as termination rather than as expected
destinations on purpose: the bug was not a wrong destination, it was a cycle.

## Acceptance Criteria

- [x] The `complete && to.name === 'welcome' → maps` rule is gone, with the reason recorded in
      the code rather than only here
- [x] An onboarded device with an expired session lands on `/welcome` and stays there
- [x] Gates are importable without a DOM, so they can be unit-tested
- [x] The desktop branch returns a decision instead of performing navigation
- [x] A test drives gate + auth to a fixpoint over every state combination and fails on a cycle
- [x] The test was confirmed to **fail** with the old rule restored, not merely to pass now
- [x] `vue-tsc`, `npm test` and `npm run build` clean

## Progress Log

- 2026-08-30 — Reported: `/` hangs on mobile, laptop fine. Checked the BFF first — `/api/me`
  answers 401 in 10 ms and the healthcheck is green — so it was not the API, and not
  `ensureReady()` blocking.
- 2026-08-30 — Found by reading the guard as a state machine rather than as code: the gate and
  the auth fallback form a cycle whenever `onboarding.complete` is true and the session is not
  valid. The laptop cannot reveal it because the desktop branch returns before the auth step.
- 2026-08-30 — Rule removed; gates extracted to `router/gates.ts`; desktop branch returns
  `LEAVE_APP` and the guard performs the navigation.
- 2026-08-30 — **Verified the test catches it**: with the old rule temporarily restored, both
  the targeted case and the exhaustive termination test fail; with it removed, all 34 tests
  pass. A regression test nobody has seen fail is only a guess that it works.
- 2026-08-30 — ✅ `vue-tsc`, 34 tests and `npm run build` clean.

  **Still worth checking on the device**, because this fixes a loop that was definitely there
  but I cannot confirm it was the *only* thing wrong: if `/` still hangs on mobile, the next
  suspects are (a) a stale service worker from the earlier device tests serving an old
  precache — hard-reload or clear site data to rule out, and (b) `isStandalone()` misreading a
  browser tab. `?nogate=1` (task 139, non-prod) disables the gates entirely and is the fastest
  way to tell whether the gate is still involved at all.
