# 329 — Offline field test on Android/Chrome

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

Split out of task 325, which ran the five offline scenarios on an installed iPhone and passed all of
them. This is the **Android/Chrome repeat**, deferred by the maintainer on 2026-09-17 so PRD 019 could
close on its iOS result.

**It is a real gap, not a formality.** The protocol passes on iOS, but three things differ enough that
"it works on iOS" is not evidence about Android:

- **IndexedDB quota behaves differently.** Chrome's eviction policy and its per-origin budget are not
  Safari's, and the outbox can hold tens of megabytes of photographs. `isQuotaExceeded` and the
  fallback-to-direct-send path in `glimt.store.ts` have never run on Chrome.
- **Android *has* Background Sync**, which this design deliberately does not use (PRD 019 §5: iOS does
  not run a backgrounded web app, so the copy must never imply background upload). Worth confirming
  that nothing on Android quietly behaves *better* than the copy says — a post that drained while
  closed would make "venter på nettet" a lie in the other direction, and would be a difference in
  behaviour between platforms that nobody documented.
- **The share sheet is not the iOS share sheet.** Task 318's save path uses `navigator.share` with a
  `File`; Chrome supports it, but where it lands is Android's business and should be seen.

## What to run

The five scenarios from task 325, unchanged. On an installed PWA (Add to Home screen), in airplane
mode:

1. Post a glimt with two photos. It appears immediately with a **Venter** badge.
2. Force-quit. Reopen offline — still there.
3. Network on **with the app open** — the outbox drains.
4. Repeat 1–2, network on **with the app closed**, then reopen — it drains on foreground, and **not
   before**.
5. Force-quit **mid-upload**. Reopen: nothing already uploaded is re-sent, no duplicate glimt.

Plus, while there:

- Tap *Gem* in the viewer — does the share sheet appear, and can the photo reach the gallery?
- Does the carousel dot track a swipe on a multi-photo card? (Fixed 2026-09-17, verified on iOS only.)

## Acceptance Criteria

- [ ] All five scenarios run on Android/Chrome, results logged here
- [ ] Zero lost media across the runs
- [ ] Pending state copy is accurate — in particular, nothing drains while the app is closed
- [ ] The save path checked on Android
- [ ] Any bug found is filed as a new task and referenced here

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-17 — Created out of task 325. The iOS pass found three bugs, all presentational and all
  fixed: the queued glimt not appearing in the feed (a PRD 019 §5 requirement miss), the queued
  card's aspect ratio, and the save button downloading the app shell. None was findable from a test
  suite. That record is the reason this task exists rather than being assumed away — the same
  protocol on a different engine is worth running.
