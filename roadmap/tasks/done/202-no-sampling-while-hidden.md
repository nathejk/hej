# 202 — Stop asking for a fix while the app is hidden

**Status:** done
**Priority:** high
**Created:** 2026-09-02
**Picked up by:** agent session (Zed)
**Started:** 2026-09-02
**Completed:** 2026-09-02

## Description

Found in the first long device report (iPhone, iOS 18.7, 22h23m, task 082). Two runs of **48
`geoerror — code=3 Timeout expired`, one every 30 seconds**, back to back:

```
21:42:53  hidden
22:35:05  start — points=21
22:35:25  geoerror — code=3 Timeout expired      ← every 30 s
…                                                  for 15 minutes
22:50:49  geoerror — code=3 Timeout expired
```

Nothing was recorded by any of them. The pattern repeats from 00:17 to 00:24.

### What is happening

The recorder is at app level on purpose (task 082): it must not stop when the user navigates away from
the map. But `sample()` never checks whether the document is **visible** — so while the app is hidden it
keeps firing every 30 s, calls `getCurrentPosition`, and iOS lets it wait out the full 20 s timeout
without servicing it. The page is alive enough to run timers and dead enough not to get a position.

Three costs, none of them buying anything:

1. **Battery.** Every attempt asks the platform for a high-accuracy fix with `maximumAge: 0` — the most
   expensive request available — on a phone in a pocket, for 20 s, twice a minute.
2. **The diagnostic log is flooded.** 96 of the 262 retained events are this one condition repeated. The
   buffer is capped, so a genuine event elsewhere is pushed out by a failure we already knew about — and
   the log is how these problems get found at all (tasks 197–201).
3. **The report reads as broken** when it is behaving as designed: 48 errors looks like a fault, and it
   is the expected consequence of a documented platform limit.

### Why skipping loses nothing

The report is the proof: across both runs, **zero points** were recorded. The feature's promise is
already "coverage of everywhere the member was *while the app was open*" (PRD 002 §11.1). A hidden
document is not open, and the platform agrees.

`visibilitychange` fires on lock and on backgrounding, and the recorder samples immediately on becoming
visible, so nothing is lost at the boundary.

### Related: the report's coverage figure needs a companion

The same report says **2% coverage** over 22 hours — 47 points against 2,688 "expected". That expectation
assumes continuous sampling across a period in which the app was mostly not running at all, so the number
is arithmetically right and rhetorically misleading: it invites the conclusion that the recorder is
broken, when what it measures is how much of a **day** a phone spends with a web app foregrounded.

Coverage *while the app was open* is the number that describes the feature. Both belong in the report.

## Acceptance Criteria

- [x] `sample()` returns early while hidden, and logs `hidden — not sampling until visible` once per run.
- [x] Becoming visible resets the flag and samples normally; `App.vue`'s existing `visibilitychange`
      handler already samples on return, so the boundary costs at most one interval.
- [x] Identical geolocation failures are collapsed — the signature is remembered and cleared on the next
      success, so a condition that recurs is still recorded, just not 48 times.
- [x] The report prints `tid med appen åben` and `dækning mens appen var åben` beside the wall-clock figure.
- [x] `isHidden` is injected on the store.
- [x] 5 new tests, including that permission loss still stops the recorder *before* the visibility check —
      the skip must not become a way to keep a revoked recorder alive.

## Progress Log

- 2026-09-02 — Task created from the 22-hour iPhone report, which is the first run long enough for this to
  show up. Worth noting the instrumentation added in tasks 197/201 is what made it visible: the same thing
  was presumably happening before and left no trace.

- 2026-09-02 — **The instrumentation found this, which is the point worth recording.** The same thing was
  presumably happening on every device before tasks 197/201 and left no trace at all; it took a 22-hour run
  and a diagnostic log to make 48 identical timeouts visible. Cheap instrumentation paying for itself.
- 2026-09-02 — **Deliberately not narrowing the feature.** It looks like a reduction in recording, and it is
  not: across both runs of timeouts, **zero points** were recorded, so nothing is lost. PRD 002 §11.1 already
  promises "coverage of everywhere the member was while the app was open", and the platform is simply
  agreeing with that wording.
- 2026-09-02 — **The coverage figure needed a companion, not a correction.** 47 points against 2,688 over 22
  hours reads as 2% and looks like a fault, when what it measures is how much of a *day* a phone spends with
  a web app foregrounded. Wall-clock coverage stays, because the gap between the two numbers *is* the
  platform limit and that is the interesting quantity — but the report now also states coverage of the time
  the app was actually open, which is the number that describes the feature.
- 2026-09-02 — Also from the same report, recorded rather than acted on: **accuracy 3.9 / 10.5 / 20 m** on an
  iPhone against 35–40 m on the Wi-Fi-only iPad, which is the GPS-versus-Wi-Fi difference measured on our own
  two devices; **8 of 28 backgroundings ended in an iOS kill** with no resume, quantifying the aggressive-kill
  behaviour task 082 described; and **quota 41,232 MB with `persisted() = true`**, a second confirmation of
  the 60%-of-disk rule.
- 2026-09-02 — ✅ Green: 5 new tests, suite 391 across 32 files, `type-check` and `build` clean.
