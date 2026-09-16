# 298 — The app never re-checks for a new build while it stays open

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

Found while testing task 290: a device sat on **build `main.85`** while `main.87` was current, and no
update banner ever appeared.

`helpers/pwa.ts` registers the service worker like this:

```ts
registerSW({ immediate: true, onNeedRefresh, onOfflineReady })
```

`registerSW` triggers exactly **one** update check — at registration, i.e. per document load. Nothing
calls `registration.update()` afterwards, and there is no interval. So the sequence is:

1. Document loads → SW registers → one check for a new `sw.js`.
2. From then on, **nothing ever looks again** for as long as that document lives.

Task 280 measured how long that is on iOS: **one document survived 47 minutes across seven
suspend/resume cycles**, including a 32-minute suspension, and iOS keeps installed PWAs alive
aggressively. So "as long as that document lives" can be hours or days of real use. A participant can
sit on a stale build indefinitely, and the only way out is the OS discarding the app.

## Why this matters more than it looks

This is the app's **only mechanism for shipping a fix during an event**. PRD 017 spent considerable
effort on an operator lever that takes effect without a release — the served `interval_seconds` — and
the reasoning there ("waiting for a redeploy to stop a load problem is not a plan") applies with more
force here: if a bug needs a code fix at 02:00, there is currently no way to get it onto devices that
are already open. The lever exists for load and not for correctness.

It also quietly invalidated part of this project's own test loop: several builds were pushed during the
PRD 017 device testing, and it is now unclear which of them the device was actually running at any
point. Task 297 concluded that the earlier double-load was an accepted update reload — that conclusion
still holds, but it means the *only* times the device picked up a new build were the times somebody
happened to accept a banner that only appears on a fresh document.

## Approach

`vite-plugin-pwa` supports this directly: `onRegisteredSW(url, registration)` hands back the
registration, and a periodic `registration.update()` is the documented recipe. Points to settle rather
than assume:

- **Interval.** Long enough to be free (this is a conditional request for one small file), short enough
  to be useful mid-event. An hour is the plugin's own example; 15–30 minutes is defensible for an app
  used for one night a year. Consider serving it, like PRD 017's other levers.
- **Only when visible and online.** The same discipline as `useFreshnessLoop`: a phone in a pocket must
  not poll, and a failed check while offline is a non-event. Reusing `useFreshnessLoop` itself is worth
  considering — it already owns "check at the moments worth checking", already debounces, and already
  stops when hidden. That would make this a *fourth* consumer of a convention rather than a new timer.
- **A check on foreground.** Arguably more valuable than any interval: a device coming out of a pocket
  is exactly when a waiting fix should be noticed.
- **Do not auto-apply.** `registerType: 'prompt'` is a deliberate choice (never reload someone
  mid-task, least of all mid-event) and this must not quietly become an auto-update. It only needs to
  make the *banner* appear reliably.
- **`Senere` still means later.** With the banner appearing more often, a dismissed update returning
  every 20 minutes would be worse than the bug. Dismissal needs to stick for a sensible while.

## Diagnosing it before changing anything

Confirm the server and client disagree, so this is not a lagging deployment:

```sh
curl -s https://hej.local.nathejk.dk/api/healthcheck | jq .system_info.version
```

That reports the **API binary's** version; the client's build id is the one on the bottom nav
(`SHOW_BUILD_ID`). Both come from the same `BUILD_VERSION` build arg, so on a correctly deployed stack
they match. If the API says `main.87` and the client says `main.85`, the client is stale and this task
is the cause. If the API also says `main.85`, nothing has been deployed and there is no bug here.

## Acceptance Criteria

- [ ] Confirmed against `/api/healthcheck` that the server is ahead of the client (not a deploy lag).
      **Still worth doing on the device** — the fix is right regardless, but the field observation
      (`main.85` vs `main.87`) has not been separated from a possible deploy lag.
- [x] A new build is noticed by an already-open app without a cold start.
- [x] The check runs on foreground, and on an interval while visible — never while hidden.
- [x] No auto-reload: the banner still waits for the user (`registerType: 'prompt'` preserved).
- [x] Dismissing with **Senere** is respected for a stated period, not re-shown minutes later.
- [x] A failed check while offline is silent and retried later.
- [x] Decided and documented: reuse `useFreshnessLoop` or a separate timer, with the reason.
- [ ] Tested on the device that found this: `main.<n>` picked up while the app stays open.

## Progress Log

- 2026-09-17 02:30 — Task created. Found on a device running `main.85` against `main.87`, with no
  banner. Cause identified by reading `helpers/pwa.ts`: `registerSW` checks once per document load and
  nothing ever checks again, which task 280's measurements show can mean hours.
- 2026-09-17 03:00 — Implemented. `helpers/pwa.ts` captures the registration via `onRegisteredSW` and
  exposes `checkForUpdate()`; the new `useUpdateCheck` composable decides when to call it.
- 2026-09-17 03:05 — **Reused `useFreshnessLoop` rather than adding a timer**, which was the main design
  decision here. It is the same question at a different cadence — "has something changed?", asked at the
  moments worth asking — and the composable already owns every answer this needed: foreground, interval
  while visible, reconnect, stop when hidden, debounce. A private timer would have re-derived all of it
  and made this the third thing polling on its own schedule, which is what PRD 017 spent its effort
  removing. It is also the **fourth** consumer of that convention, which is the first real evidence it
  generalises rather than merely being reusable in principle.
- 2026-09-17 03:10 — 15 minutes, debounce 60 s. Deliberately **not** served from `/api/config`, unlike
  PRD 017's interval: that lever exists so an operator can shed load mid-event, whereas this one
  addresses "we cannot ship a fix at all", and a remote switch whose only effect is to reinstate the bug
  is not a lever worth having. The cost is a conditional request for one small file — a few hundred
  devices at 15 minutes is well under one request per second.
- 2026-09-17 03:15 — It only makes the banner appear; it never activates or reloads. `registerType:
  'prompt'` stays, because reloading the app under a patrol mid-navigation would be worse than the bug
  being fixed. "Senere" still sticks for the session: `updateAvailable` latches true, and `UpdatePrompt`
  only clears `dismissed` on a false→true transition, so a dismissed banner does not return every 15
  minutes.
- 2026-09-17 03:20 — **A test found a second, latent bug.** The "survives a failing check" case produced
  an *unhandled promise rejection*: `useFreshnessLoop` awaited `spec.check()` in a `try/finally` with no
  `catch`, and every trigger calls it as `void check()`. Nothing in production hits it today (all four
  consumers catch internally, and the spec's contract says a check must not throw) — but the symptom of
  breaking that contract would have been an unhandled rejection with no stack naming the loop and no clue
  which dataset caused it. The loop now contains and logs it, and stays alive. Contract unchanged; the
  failure mode is just no longer silent-and-remote.
- 2026-09-17 03:25 — `virtual:pwa-register` does not exist under vitest (the PWA plugin is not in that
  pipeline), so the spec mocks `@/helpers/pwa` at the module seam. Every test injects its own `check`
  anyway — this file is about *when* the question is asked, which was the entire defect.
- 2026-09-17 03:30 — 59 files / 740 tests green, type-check clean, production build clean. Left the two
  device-confirmation criteria open: the fix is right on its own terms, but nobody has yet watched an
  open app pick up a build, and the original `main.85`/`main.87` observation has not been separated from
  a possible deploy lag.
