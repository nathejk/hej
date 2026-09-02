# 198 — Fall back to a coarse fix on devices without GPS

**Status:** done
**Priority:** high
**Created:** 2026-09-02
**Picked up by:** agent session (Zed)
**Started:** 2026-09-02
**Completed:** 2026-09-02

## Description

Task 197 made the location failure *visible*. This is about making it not happen.

The device from the 2026-09-02 run turns out to be an **iPad (6th generation), iPadOS 17.7.10, model
MR7F2KN/A** — a **Wi-Fi-only** iPad. Wi-Fi-only iPads have **no GNSS receiver at all**: there is no GPS
chip to ask. Their only positioning source is Apple's Wi-Fi network database, which is coarse, slower,
and fails outright where the surrounding networks are unknown.

Every geolocation call in the app asks for `enableHighAccuracy: true`. On this class of device that is a
request the hardware cannot satisfy, and WebKit's answer is a slow `POSITION_UNAVAILABLE`, a `TIMEOUT`,
or — as observed — nothing at all. So the app asks for the one thing the device cannot do, and then has
nothing to fall back on.

**This is not an exotic device.** An iPad is a plausible crew tool: a leader's tablet, something at HQ,
a spare in a car. It is also plausibly what a family lends a participant. The app's baseline is
iPadOS 16.4+, and an iPad 6th generation on 17.7.10 is comfortably inside it — iPadOS 17 is the last
major version that model receives, so this is not a device that will update its way out of the problem.

### What high accuracy is actually for

The position track (PRD 002 §11.1) wants precision: 30 s sampling at walking pace puts points 33–50 m
apart, and a GPS error of 10–30 m is already most of that. So `enableHighAccuracy: true` is right as a
*first* choice and wrong as the *only* one — a coarse Wi-Fi fix of ±500 m still answers "which end of
the forest is this patrol in", which is the question that matters at 03:00, and is infinitely better
than the blank marker the iPad got.

## Acceptance Criteria

- [x] A failed high-accuracy request is retried **once**, coarse (`GEO_COARSE`), before reporting
      failure. A denial is never retried.
- [x] The coarse attempt gets 15 s and will accept a two-minute-old fix.
- [x] A coarse fix is a normal success: position, `granted`, no warning, no extra copy.
- [x] `location.coarse` is set and logged as a `nofix` diagnostic event when the coarse path answered.
- [x] Tests cover all four paths, plus an assertion that the coarse options really are the more patient
      ones — so a future edit cannot quietly make the fallback identical to the first attempt.
- [x] `.rules`' baseline unchanged: iPadOS 17.7.10 is well inside iPadOS 16.4+. This widens what works
      inside the baseline rather than moving it.

## Progress Log

- 2026-09-02 — Task created after the device's model number identified it as a Wi-Fi-only iPad, which
  reframed task 197's symptom: not only was the failure invisible, it was a failure we asked for by
  demanding GPS accuracy from hardware with no GPS.

- 2026-09-02 — **The device identifies the whole class of problem.** iPad (6th generation), model
  MR7F2KN/A — a Wi-Fi-only iPad, so there is **no GPS chip in it at all**. Its only positioning source is
  Apple's Wi-Fi network database: coarser, slower, and capable of simply not knowing. Every geolocation
  call in the app asked for `enableHighAccuracy: true`, which on that hardware is a request that cannot
  be satisfied — so the app was asking for the one thing the device cannot do, with nothing to fall back
  on. Task 197 made that failure visible; this makes it stop happening.
- 2026-09-02 — Worth noting the device is also **end of life at iPadOS 17**: the 6th generation receives
  no newer major version, so this is not a problem that updates itself away. It is a plausible crew tool
  — a leader's tablet, something at HQ, a spare in a car — and comfortably inside the app's stated
  baseline.
- 2026-09-02 — **A denial is never retried**, and that is a deliberate exclusion rather than an
  optimisation: the answer cannot change without a trip to Settings, and on some platforms repeated
  requests are precisely what gets a permission permanently blocked. Retrying would risk making the
  situation worse than the one being recovered from.
- 2026-09-02 — **When both attempts fail, the reported cause is the coarse attempt's.** What the user is
  told should describe the last thing actually tried, not the first.
- 2026-09-02 — **Not changed: the position track still asks for high accuracy only** (`track.store`). It
  is a real question and it deserves its own decision rather than being swept along, because the two
  surfaces present uncertainty very differently: the map draws an accuracy circle, so a ±500 m fix is
  *visibly* approximate, whereas a recorded track is drawn as a **line**, which implies a precision a
  Wi-Fi fix does not have. A coarse track might be better than no track — or it might be a misleading
  artefact shown to a team after the race (PRD 011). Raised as an open question on PRD 002 §11.1 rather
  than decided here.
- 2026-09-02 — ✅ All criteria complete. 5 new tests (19 in this store's spec); suite 376 across 31
  files; `type-check` and `build` clean. Still unverified on the iPad itself: what this changes is that
  the coarse path now exists and, if it answers, the map will show a position with an honest circle.

- 2026-09-02 — **Corrected by measurement.** This task asserted that high accuracy is "a request the
  hardware cannot satisfy" on a Wi-Fi-only iPad. The first real report from that device shows fixes at
  **35–40 m** accuracy, `placeringstilladelse: granted`, 12 points recorded — so Wi-Fi positioning on it is
  not merely usable, it is good enough that the distinction from GPS barely matters for this app's purpose.
  The claim should have been narrower: high accuracy cannot be satisfied *by GPS*, and WebKit may take
  longer or fail where the network is unknown. The coarse fallback is still worth having for the case where
  it does fail, but it is insurance rather than the load-bearing fix I described.
