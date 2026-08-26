# 082 — Record the position track locally

**Status:** open
**Priority:** high
**Created:** 2026-08-26
**Picked up by:**
**Started:**
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
was open**, not a continuous route. That is enough to validate the concept and the whole
pipeline; it is not enough to call it "the entire track" (see task 086).

### Consequence for where this code lives

`MapsView.vue` currently calls `location.stopWatch()` on `document.hidden` and on unmount.
That is correct for a map marker and wrong for a track recorder: as it stands, recording
would stop merely by navigating away from `/maps`.

**The recorder belongs at app level** — running while signed in and permission is granted —
not in the map view. Drawing the live marker and recording the track are separate concerns
that happen to share a data source. Keep the map's visibility-suspend behaviour for the
marker; do not let it govern the track.

## Sizing

At 30 s sampling a 12-hour race is ~1,440 points, about **195 KB** per person including
upload framing. Trivial against any plausible quota — even the pre-iOS-17 ~1 GB ceiling —
so there is no reason to be clever about compaction.

## Acceptance Criteria

- [ ] **Verified on a real device**: what happens to recording when the app is
      backgrounded, when the screen locks, and when the user switches apps. Result recorded
      here before the rest is built
- [ ] Positions are appended to IndexedDB as they arrive, while permission is granted
- [ ] Recording is **not** tied to the map view's lifecycle — navigating away from `/maps`
      does not stop it
- [ ] Sampling at 30 s, with the acquisition mode chosen deliberately: continuous
      high-accuracy `watchPosition` is not obviously right for a 30 s cadence, and battery
      cost over a night is the deciding factor
- [ ] Battery cost measured on a device over a representative period, result recorded here
- [ ] The track survives a page reload, backgrounding, and the app being killed
- [ ] `navigator.storage.persist()` is requested, and the outcome is observable rather
      than assumed
- [ ] Each point carries at least a timestamp, lat, lng and accuracy; (person, timestamp)
      identifies a point, so a duplicate is detectable (needed by 083)
- [ ] Recording stops when permission is revoked, and nothing is recorded before it is
      granted
- [ ] Storage growth is bounded or at least observable — a runaway writer on a stationary
      phone must not fill the quota that portraits and tiles also draw on
- [ ] `QuotaExceededError` is handled rather than thrown into the void

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
- 2026-08-26 — Sampling interval decided (30 s) and the background-recording limitation
  documented before any code was written. The limitation is the substance of this task; the
  sampling rate turned out to be the easy half.
