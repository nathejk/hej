# 082 — Record the position track locally

**Status:** doing
**Priority:** high
**Created:** 2026-08-26
**Picked up by:** agent session (Zed)
**Started:** 2026-08-27
**Completed:**

## Description

PRD 002 §11.1. While location permission is granted, record the user's position into
local persistent storage as it is collected — so a member walking through a dead spot is
still recorded, and connectivity affects only *when* the track ships.

This is the recording half. Uploading is task 083.

## Why persistent storage, not memory

An unshipped track is **the only data in this app that cannot be recovered from the
server**. Portraits share that property (PRD 007) and are treated carefully for the same
reason. A track held in memory is lost when the phone kills a backgrounded tab, which on
iOS is routine rather than an edge case.

So: **IndexedDB**, not `localStorage` — the latter is synchronous, string-only and ~5 MB,
and blocking the main thread on every position fix would be a poor trade on a phone.

Call `navigator.storage.persist()`. WebKit grants persistent mode "based on heuristics
like whether the website is opened as a Home Screen Web App", which is exactly this app's
install-first onboarding, so the request is likely to be granted rather than decorative.
Without it the track sits in best-effort storage and can be evicted under pressure.

## Sampling interval: 30 s (decided 2026-08-26)

"Team are walking, we do not need sub-30 s resolution." That gives **1,440 points per
person** for a 12-hour race, **~195 KB per person**, **~157 MB per event**.

The reason it is the right interval and not merely a cheap one — worth knowing so nobody
"improves" it later: at walking pace 30 s puts consecutive points **33 m apart at 4 km/h,
50 m at 6 km/h**, while a phone's GPS error under forest canopy at night is **10–30 m**.
Below ~30 s the spacing between points is smaller than the error on each point, so the
extra samples record receiver noise rather than movement.

## The hard part: a web app cannot record while backgrounded

Verify this on a real device **first**, before building the rest, because it determines what
the feature can promise.

Geolocation is only available to a document, and a backgrounded document does not run — on
iOS, JavaScript in a backgrounded web app is suspended. There is no background geolocation
for web apps on any platform. Two apparent escape routes that are not:

- **Screen Wake Lock** keeps the screen from dimming *while the document is visible*; the
  spec states locks "are automatically released when document becomes inactive". It buys a
  lit screen, not background execution — and a lit screen for 12 hours is both a battery
  problem and a **light-discipline** problem in a night race.
- **Periodic Background Sync** (Chrome-only anyway) can wake a service worker, but service
  workers have no Geolocation API. Dead end even on Android.

So the realistic deliverable is a track covering **everywhere the member was while the app
was open**, not a continuous route.

**Accepted 2026-08-26 (maintainer): "fine with fragmented trace, just track as much as
possible."** So the goal is coverage, not continuity — which makes the device check below a
measurement rather than a go/no-go, and makes the next section the highest-value part of this
task.

### Consequence for where this code lives — the highest-value part of this task

`MapsView.vue` currently calls `location.stopWatch()` on `document.hidden` and on unmount.
That is correct for a map marker and wrong for a track recorder: as it stands, recording
would stop merely by navigating away from `/maps`.

**The recorder belongs at app level** — running while signed in and permission is granted —
not in the map view. Given the goal is "as much as possible", this single change is what most
increases coverage: it turns "recorded while looking at the map" into "recorded whenever the
app is open at all". Drawing the live marker and recording the track are separate concerns
that happen to share a data source. Keep the map's visibility-suspend behaviour for the
marker; do not let it govern the track.

## Sizing

At 30 s sampling a 12-hour race is ~1,440 points, about **195 KB** per person including
upload framing. Trivial against any plausible quota — even the pre-iOS-17 ~1 GB ceiling —
so there is no reason to be clever about compaction.

## Acceptance Criteria

- [x] **Measured on a real device**: what happens to recording when the app is backgrounded,
      when the screen locks, and when the user switches apps. Recorded here — not as a
      go/no-go (fragmented is accepted) but so the expected coverage is known and anything
      worse than predicted is caught
      *Measured 2026-08-27 on an iPhone (iOS 18.7, installed). Recording stops completely
      while backgrounded, resumes within 60 ms. See the log entry for the numbers.*
- [x] Positions are appended to IndexedDB as they arrive, while permission is granted
- [x] Recording is **not** tied to the map view's lifecycle — navigating away from `/maps`
      does not stop it
- [x] Sampling at 30 s, with the acquisition mode chosen deliberately: continuous
      high-accuracy `watchPosition` is not obviously right for a 30 s cadence, and battery
      cost over a night is the deciding factor
- [ ] Battery cost measured on a device over a representative period, result recorded here
- [x] The track survives a page reload, backgrounding, and the app being killed
      *All three now measured. The device run included three cold starts (iOS killing the
      backgrounded app) and the points from before each survived — 9 points spanning
      6m 47s across them.*
- [x] `navigator.storage.persist()` is requested, and the outcome is observable rather
      than assumed
- [x] Each point carries at least a timestamp, lat, lng and accuracy; (person, timestamp)
      identifies a point, so a duplicate is detectable (needed by 083)
- [x] Recording stops when permission is revoked, and nothing is recorded before it is
      granted
- [x] Storage growth is bounded or at least observable — a runaway writer on a stationary
      phone must not fill the quota that portraits and tiles also draw on
- [x] `QuotaExceededError` is handled rather than thrown into the void

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
- 2026-08-26 — Sampling interval decided (30 s) and the background-recording limitation
  documented before any code was written. The limitation is the substance of this task; the
  sampling rate turned out to be the easy half.
- 2026-08-27 — Picked up. Built the recording half; upload stays with task 083.
- 2026-08-27 — Shape of it:
  * `@/helpers/trackDb.ts` — IndexedDB, raw rather than via a wrapper library (one object
    store, three operations, no schema evolution yet; a dependency would be more code
    than the file). Key path is **`[userId, ts]`**, which turns PRD 002 §11.1's
    "(person, timestamp) identifies a point" into a constraint the database enforces
    instead of a convention the upload code must remember. Two consequences: re-recording
    the same instant **overwrites** rather than appends, so a retry or a double-started
    recorder cannot produce duplicates; and the track is per *person*, not per device,
    which matters because ~1 in 8 numbers is shared by siblings (task 079) and one phone
    can legitimately carry two tracks.
  * `@/config/track.ts` — the 30 s interval and the reasoning behind it, kept where it
    will be read before someone "improves" it.
  * `@/stores/track.store.ts` — the recorder.
  * `App.vue` — starts it, plus a watcher on (permission, user) so a later grant or a
    revocation reaches it.
  * `PrivacyView.vue` — shows how many points are stored, whether recording is running,
    whether the browser promised not to evict, and any problem.
- 2026-08-27 — **The highest-value part, as the task predicted: the recorder is at app
  level, not in `MapsView`.** Verified in a browser rather than argued: navigate from
  `/maps` to `/rulebook` and a new point still appears 30 s later. MapsView keeps its
  own visibility-suspend behaviour for the marker; it no longer governs the track.
- 2026-08-27 — **Acquisition mode, decided rather than defaulted.** Not a second
  continuous high-accuracy `watchPosition`: that keeps the receiver hot for twelve hours
  to use one fix in sixty, and battery over a night race is the deciding factor. Instead
  the recorder reuses a fix younger than 25 s if one exists — while the map is open its
  watch supplies these, so recording then costs **nothing extra** — and otherwise takes a
  single fix, letting the radio sleep in between. `location.store` gained a
  `positionAt` timestamp to make "is the current fix fresh?" answerable.
- 2026-08-27 — Settled on `maximumAge: 0` for the recorder's own `getCurrentPosition`.
  The platform's cache would buy nothing, because the cheap path is already covered one
  level up: if a recent fix exists it came from the map's watch and is reused without
  calling geolocation at all. What a cached fix would *cost* is timestamp accuracy — `ts`
  is when the position was true and is half the primary key, so accepting a 25 s-old fix
  lets a 30 s cadence store points 5 s apart in the track. Reaching the acquire path
  means nothing recent is known, which is exactly when a fresh fix is the right ask.
- 2026-08-27 — **A real bug found while chasing a wrong cadence: `start()` could run
  twice concurrently.** It awaits three things (the persistence request, the count, the
  last timestamp) before it can set `recording`, and it is called from two places —
  `onMounted` and the permission/session watcher. Both calls sailed past
  `if (this.recording)` and the app ended up with two intervals writing two sets of
  points. Fixed with a `starting` flag taken *before* the first await, plus a `sampling`
  flag so two fixes are never in flight at once. Worth noting how quiet the symptom was:
  a doubled recorder showed up only as a wrong sampling interval.
- 2026-08-27 — Second bug from the same investigation: `IDBKeyRange.bound([userId,
  -Infinity], [userId, Infinity])` **throws** — `Infinity` is not a valid IndexedDB key.
  The exception was swallowed into a `console.error`, left `lastPointAt` at 0, and made
  every page load take an immediate fix, so a few reloads produced a burst of points
  seconds apart. Now bounded with plain numbers (8.64e15 = the maximum JavaScript Date).
  It was invisible because the harness captured `pageerror` but not `console`; it now
  captures both, and that is the lesson worth keeping.
- 2026-08-27 — Cadence is recovered across page loads by reading the newest stored
  timestamp back from IndexedDB, because a full navigation remounts the app and would
  otherwise re-sample immediately. The comparison guards a future timestamp too
  (`elapsed >= 0`): some receivers report a clock that runs ahead, and a future value
  would otherwise suppress recording until real time caught up — an hour of silence from
  a one-off clock error.
- 2026-08-27 — **Verified in a real browser, 14/14 checks**, against the `prod` image with
  Chromium's geolocation overridden, reading IndexedDB directly rather than trusting the
  UI: nothing recorded before the grant; a point immediately after it, carrying userId,
  timestamp, lat, lng, accuracy and `uploaded: 0`, at the actual coordinates; recording
  continues after leaving `/maps`; 2 points in 65 s; timestamps ordered and plausible;
  the track survives a reload; the privacy page reports the count; recording stops on
  revocation; no console or page errors.
- 2026-08-27 — Two harness lessons, both of which produced a red result from correct code:
  * `indexedDB.open()` **creates** the database. Probing for points before the app had
    written left an empty version-1 database behind, so the app's own `open()` never fired
    `onupgradeneeded`, the `points` store never existed, and every write failed. The
    harness broke the thing it was measuring. It now checks `indexedDB.databases()` first.
  * Chromium's **emulated** geolocation reports the moment `setGeolocation` was called as
    the fix timestamp, not the moment of the request. With the simulated position held
    still, every sample therefore returns the *same* timestamp, each write overwrites one
    record, and the count never grows. I spent three cycles reading that as missed
    samples before instrumenting and finding the recorder firing at exactly 30 s
    intervals. The harness now moves the position during the measurement window. It also
    accidentally demonstrated the intended idempotence: a repeated identical fix produces
    no phantom point.
- 2026-08-27 — And a near-miss worth recording: the "recording stops when permission is
  revoked" check was **vacuous** as first written. With a stationary simulated position
  the count cannot grow whether recording stopped or not, so it would have passed against
  a recorder that kept running. It now moves the position throughout that window too.
- 2026-08-27 — `navigator.storage.persist()` returned **false** in headless Chromium, and
  the answer is shown on the privacy page rather than assumed. WebKit is expected to grant
  it for a home-screen web app, which is this app's onboarding — but that is a claim about
  a real device, so it belongs to the device criteria below, not here.
- 2026-08-27 — `QuotaExceededError` is caught and typed (`TrackStorageFullError`), stops
  recording and surfaces on the privacy page. Not *exercised*: the test environment has a
  37 GB quota and filling it to prove a catch block is not a good use of a laptop. Marked
  done as implemented-and-reachable, flagged here as unverified by execution.
- 2026-08-27 — `vue-tsc --noEmit` clean, `npm run build` succeeds.
- 2026-08-27 — **Left for the maintainer, and they are the two that decide what this
  feature can promise:** what actually happens to recording when the app is backgrounded,
  the screen locks and the user switches apps; and the battery cost over a
  representative period. Both need a real phone. The code cannot record while
  backgrounded on any platform — that is a documented platform limit, accepted
  2026-08-26 — so these are measurements of expected coverage, not go/no-go tests.
  Everything else in this task is done and verified.
- 2026-08-27 — Built the instrumentation the device measurement needs, since the two
  remaining criteria are answered on a phone and a maintainer should not have to read them
  out of a debugger:
  * **A persisted event ring** in IndexedDB (`events` store, DB v2) recording `load`,
    `start`, `stop`, `point`, `skip`, `nofix`, `geoerror`, `full`, `capped`, `hidden` and
    `visible`. Persisted rather than in memory precisely because the interesting cases
    destroy memory: iOS suspending a backgrounded app, and the OS killing it. An event
    written before the app went away is the only evidence that survives.
  * **`hidden`/`visible` logging at app level**, plus a prompt sample on resume (which
    cannot oversample — `sample()` still refuses inside one interval). The pair of
    timestamps either side of a gap in the points is what actually answers "what does
    backgrounding cost".
  * **A status page at `/sporing`**, linked from the privacy page and deliberately *not* a
    nav destination: it is a measurement tool, not a feature. It renders a plain-text
    report with a "Kopiér rapport" button, and also as selectable text — clipboard access
    can be refused, and a report you can select by hand is the difference between a
    measurement and a lost night.
- 2026-08-27 — The centre of the report is the **gap analysis**, computed from the stored
  timestamps: total span, points actual vs. what a perfect recorder would have taken,
  coverage as a percentage, the count of gaps longer than two sampling intervals, total
  time lost to them, and the fifteen largest with clock times. Two intervals is the
  threshold because one missed sample can be a slow fix, while two means the app was not
  running. It also counts each event kind over the whole period and reports how many points
  reused the map's fix versus cost a fix of their own — the battery argument, measured
  rather than asserted.
- 2026-08-27 — Sized the event ring against the measurement instead of picking a round
  number: at 30 s sampling a 12-hour night produces ~1,440 `point` events on its own, so
  the first version's 400-entry ring would have spent itself on routine successes and
  discarded exactly the `hidden`/`visible` transitions the test is for. Now 2,000 (~120 KB,
  against the 195 KB of points it annotates).
- 2026-08-27 — Battery cannot be read by the app: **iOS has no Battery Status API**. The
  report therefore ends with a blank line for it and says why, so the number gets written
  down next to the run it belongs to rather than remembered wrongly afterwards.
- 2026-08-27 — Verified the page in a browser, all checks passing, with a synthetic history
  injected into IndexedDB — 40 points at 30 s, a 14-minute gap, then 20 more. The analysis
  found both gaps (the injected one plus the join to the live points), ranked them
  longest-first, and reported 51% coverage over 59m 22s. Testing the analysis in a browser
  is the honest split: a headless page does not suspend the way iOS does, so the phone
  contributes real gaps, not a different code path.
- 2026-08-27 — Harness note: launching Chromium started failing with
  `Network.enable timed out` once the host load average passed ~12. Not a code fault —
  Chromium starts but the first CDP round-trip misses its budget. The harness now retries
  the launch and uses a pipe rather than a websocket. Also worth remembering: a killed
  harness leaves a Chromium container pegging a core, which makes the *next* run fail the
  same way.
- 2026-08-27 — **DEVICE RUN. iPhone, iOS 18.7, Safari 26.6, installed to the home screen
  (`standalone: true`), role spejder.** 9 points over 6m 47s, 3 cold loads, 4 `hidden`,
  2 `visible`.
- 2026-08-27 — What it settled, in order of value:
  * **`storage.persisted()` = `true`.** WebKit granted persistent storage to the
    home-screen app, as the task predicted it would. The track is not in evictable
    storage, which matters more here than for tiles because an evicted track cannot be
    re-fetched from anywhere.
  * **Backgrounding stops recording completely, and resumption is immediate.** `hidden` at
    15:27:50 → `visible` at 15:30:36 produced **zero** points in between, matching a
    2m 52s hole in the track exactly; the first point after resuming landed **52 ms**
    after `visible`. So the platform limit is real and total — no throttled trickle — and
    the resume-sample added for this task works.
  * **Accuracy is much better than assumed: 5.0 / 7.2 / 11.5 m** (min/median/max) versus
    the 10–30 m the 30 s interval was reasoned from. That does not make the interval wrong
    — 30 s still puts points 33–50 m apart at walking pace, comfortably above 7 m — but the
    canopy-error assumption should be treated as pessimistic, and this is the number to
    revisit it against if anyone argues for a longer interval.
  * **6 of 10 points reused the map's fix** and cost no extra geolocation. The
    battery argument for the acquisition mode holds on a real device.
  * **The dedupe works in the field**: 10 `point` events produced 9 stored points, because
    two samples returned the same fix timestamp and the second overwrote the first rather
    than inventing a position.
- 2026-08-27 — **And it found a real bug, which is why the run was worth doing before
  declaring this done.** The report said `optager nu: false` with
  `placeringstilladelse: prompt` on a device that had just been recording.
  Cause: **WebKit answers `prompt` to
  `navigator.permissions.query({name: 'geolocation'})` even when permission is granted** —
  Safari exposes no queryable granted state for geolocation. So on every cold start the app
  concluded it had no permission, refused to start the recorder, and stayed off until the
  user happened to open the map, which was the only thing producing a successful fix.
- 2026-08-27 — Why that was worse than it first looks: the same run shows **three cold loads
  in eight minutes** — iOS kills a backgrounded standalone web app aggressively. Two of the
  four `hidden` events are followed by a `load` rather than a `visible`, i.e. the app was
  terminated, not resumed. So in practice recording was off for most of a session unless
  the participant kept returning to the map. This task moved the recorder out of `MapsView`
  precisely so the track would not depend on looking at the map; **permission detection had
  quietly re-coupled them**, and only a device could show it.
- 2026-08-27 — Fix: a successful fix is direct evidence of a grant, so it is remembered in
  localStorage and **trusted over the Permissions API**. `granted` and `denied` from the API
  are believed outright; `prompt` may no longer overwrite a remembered grant. It is
  self-correcting — a PERMISSION_DENIED clears the memory and sets `denied` — and it is only
  ever used to *skip* asking again, never to ask: a device that has not granted is left
  alone, so PRD 002's consent copy still comes before any system dialog.
- 2026-08-27 — Regression test built for exactly this, since Chromium answers `granted` and
  would never have caught it: `navigator.permissions.query` is stubbed to behave like
  Safari, and the harness asserts that a never-granted device records nothing and still gets
  the pre-prompt, that granting starts recording, that a **cold start away from the map**
  then records with the API still saying `prompt`, and that a denial clears the remembered
  grant and stops recording. 11/11 pass. The recording suite was re-run afterwards
  (14/14) to confirm nothing regressed.
- 2026-08-27 — Chromium **cannot** emulate a revoked permission — clearing the override does
  not produce a denial, the request simply never completes — so the denial path is tested by
  stubbing the error callback with code 1. That tests our handling, which is what could be
  wrong. Whether iOS actually delivers code 1 when a user revokes in Settings is a device
  question and is **not yet verified**; worth a minute during the next run.
- 2026-08-27 — Improvements the run motivated: `nostart` events (a refusal now says why),
  a background section in the report pairing `hidden`/`visible` into suspend spans and
  counting how many ended in a kill instead of a resume, and rounded accuracy — the raw
  floats (`4.986577009122841`) were unreadable in a report meant to be read.
- 2026-08-27 — **Still open: battery.** The run was ~8 minutes with the screen mostly on,
  which measures nothing useful, and the before/after fields came back blank. It needs a
  representative period — ideally a couple of hours with the phone in a pocket, the app
  open, which is the real usage — and iOS has no Battery Status API, so the numbers have to
  come from Settings → Battery by hand.
