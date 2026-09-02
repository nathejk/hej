# 199 — Race a watch against getCurrentPosition, because iPadOS answers one and not the other

**Status:** done
**Priority:** high
**Created:** 2026-09-02
**Picked up by:** agent session (Zed)
**Started:** 2026-09-02
**Completed:** 2026-09-02

## Description

Evidence from the second iPad run (2026-09-02, iPad 6th gen, iPadOS 17.7.10, installed to the home
screen), which is the first time the app has been able to describe its own failure:

| step | result |
|---|---|
| Apple Kort, before | **"Lokalitetstjenester er slået fra"** — Location Services off for the whole device. Root cause of the original silence, now understood. |
| Location Services enabled, Apple Kort again | **Locates the iPad correctly** in Vanløse. So Wi-Fi positioning on this device works. |
| Our app, tapping "Slå placering til" | **"Venter på telefonen…"**, then after the guard: **"Der kom ikke noget svar fra telefonen."** |

So with Location Services on, and the platform demonstrably able to find the device,
**`getCurrentPosition` in our installed standalone PWA never calls either callback.** Not an error —
silence. Task 197's wall-clock guard is the only reason we know.

### The flaw this exposes in task 198

The coarse fallback is triggered from the precise attempt's **error callback**. On this device there is no
error callback — there is nothing. So the fallback never runs, and the one device it was written for is
the one device it cannot help. A retry chained off an error handler cannot rescue a call that does not
fail; it just waits with it.

### Why racing a `watchPosition` is the fix

`watchPosition` is a *different* code path in WebKit, and there is a long history of iOS home-screen web
apps where the watch delivers while the one-shot hangs. We already use `watchPosition` for the live
marker, so this is not a new dependency — it is the call the map actually wants, arriving sooner.

There is also an ugly coupling to remove. `accept()` today does:

```
request()  →  on success only  →  watch()
```

So a hung one-shot blocks the app from ever starting the watch, even though the watch is what the map
needs and might well have worked. The one-shot is the *less* important call and it is gatekeeping the more
important one.

**This is a hypothesis, not a certainty** — the run that proves it is the next one, which is why the
strategy that answers is written to the diagnostic log.

## Acceptance Criteria

- [x] A position request races three strategies and takes the first answer: precise one-shot, a watch, and
      a coarse one-shot after `GEO_COARSE_AFTER_MS` (6 s).
- [x] Nothing is chained off another strategy's error callback, so a hang cannot hold back the others.
- [x] A refusal from any strategy ends the race immediately.
- [x] A winning watch is **kept** and adopted as `watchId` — it is the subscription the map is about to
      need.
- [x] Losing strategies are cleaned up: the watch is cleared, both timers are cancelled, and a watch is
      not even started if something answered first.
- [x] The winning strategy is logged (`fix via precise|coarse|watch`).
- [x] `MapsView` starts the watch regardless of the one-shot's outcome.
- [x] Tests cover all of it: 27 in this store's spec, including the iPad case and the regression below.

## Progress Log

- 2026-09-02 — Task created from the second iPad run. Task 197 is what made this diagnosable at all: the
  app now says which of the four causes it hit, and it said `stuck` — neither callback, ever.

- 2026-09-02 — **The second iPad run is what designed this.** With Location Services enabled and Apple Kort
  locating the device correctly, our `getCurrentPosition` still produced neither callback — the app said
  "Der kom ikke noget svar fra telefonen", which is task 197's `stuck` guard firing after 25 s. So the
  platform can find the device and the one-shot cannot. That is the whole justification for racing a watch:
  it is a different code path in WebKit, iOS home-screen web apps have a long history of the watch
  delivering while the one-shot hangs, and we already use it for the live marker.
- 2026-09-02 — **Task 198's fallback was unreachable on the one device it was written for**, and this run
  proved it: the coarse retry hung off the precise attempt's *error* callback, and that device produces no
  error. A fallback chained to a failure cannot rescue a call that never fails — it just waits alongside
  it. Everything is now started independently.
- 2026-09-02 — **A regression I nearly shipped, caught by the existing tests.** Making the race ignore
  non-denied errors is right — one strategy failing says nothing about the others — but taken literally it
  means a device that fails *fast* (location switched off, every call erroring at once) waits out the full
  25 s guard before being told anything. That is precisely the silent dead-button behaviour task 197 exists
  to prevent. So the race counts outstanding attempts: when they all fail it pulls the coarse attempt
  forward immediately, and when that fails too it reports at once. Three old tests failed on this and were
  right to.
- 2026-09-02 — **A real leak found while fixing the tests, not a test artefact.** The winning-watch check
  originally read `this.strategy === 'watch'`, and the id is only known when `watchPosition` *returns* — so
  a callback firing during the call would lose the id and leave a high-accuracy subscription running that
  nothing could ever clear, on a battery that has to last the night. Now tracked with a flag, the id is
  adopted after the call if the watch won, and the watch is not started at all if the race is already over.
  Nothing in the API promises the callback is asynchronous; a real browser happens to be.
- 2026-09-02 — **Also removed the coupling in `MapsView`:** `accept()` used to start the watch only on the
  one-shot's success, so a hung one-shot prevented the app from ever starting the subscription the map
  actually needs — the less important call gatekeeping the more important one. It now starts whenever
  permission is not denied.
- 2026-09-02 — ✅ Green: suite 384 across 31 files, `type-check` and `build` clean. **The hypothesis is
  still a hypothesis.** If the next iPad run logs `fix via watch`, it is confirmed and this is the fix. If
  it logs `stuck` again, then iPadOS is refusing both paths and the next thing to examine is whether
  Safari's per-site location setting for `hej.nathejk.dk` is set to Deny — Location Services being on
  device-wide does not imply this origin is allowed.
