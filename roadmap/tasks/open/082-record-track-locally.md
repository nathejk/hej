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

## Sizing

At 10 s sampling a 12-hour race is ~4,320 points, about **409 KB** per person including
upload framing (PRD 002 §11.1). Trivial against any plausible quota — even the
pre-iOS-17 ~1 GB ceiling — so there is no reason to be clever about compaction yet.

## Sampling interval is still open

PRD 002 §11.1 prices it: 5 s costs 33× the whole domain stream per race, 30 s costs 9×.
It trades route fidelity, bytes retained, and **battery over a 12-hour night in which the
phone is also a torch and a comms device**. The existing `watchPosition` subscription uses
`enableHighAccuracy: true` with `maximumAge: 5000`, which is the expensive end.

Recommendation to validate here: sample at **10–15 s**, or on movement beyond a distance
threshold, and measure battery on a real device. Do not fix the interval on reasoning
alone.

## Acceptance Criteria

- [ ] Positions are appended to IndexedDB as they arrive, while permission is granted
- [ ] The track survives a page reload, backgrounding, and the app being killed
- [ ] `navigator.storage.persist()` is requested, and the outcome is observable rather
      than assumed
- [ ] Each point carries at least a timestamp, lat, lng and accuracy; (person, timestamp)
      identifies a point, so a duplicate is detectable (needed by 083)
- [ ] A sampling policy is implemented and its interval is **measured** on a device for
      battery cost, with the result recorded here
- [ ] Recording stops when permission is revoked, and nothing is recorded before it is
      granted
- [ ] Storage growth is bounded or at least observable — a runaway writer on a stationary
      phone must not fill the quota that portraits and tiles also draw on
- [ ] `QuotaExceededError` is handled rather than thrown into the void

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
