# 280 — Device check: which events fire on return to a home-screen PWA

**Status:** open
**Priority:** high
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 phase 1, and its first task: it decides what the sync loop listens to, so
every other frontend task in the PRD assumes an answer.

`useFreshnessLoop` currently listens to `visibilitychange` and `online` only. That is
enough for a browser tab, but the app ships as an installed home-screen PWA, and
returning to one from the iOS app switcher — or a bfcache restore after navigating
away — does not reliably present as a `visibilitychange`. If it does not, the whole
feature *appears* to work (it works on a cold start and on a tab switch) while
silently failing on the most common way the app is actually resumed during an event.

**This task needs a physical device and cannot be completed by an agent.** It is a
measurement, not an implementation.

## What to measure

On an **installed home-screen PWA** on iOS/iPadOS (baseline: Safari 16.4+), and on
Android Chrome for comparison, log every one of the following with a timestamp and
`document.visibilityState`:

- `visibilitychange`
- `pageshow` (record `event.persisted`)
- `pagehide`
- `focus` / `blur` on `window`
- `resume` (if it fires at all)

Then record which of them fire for each resume path:

1. Lock the screen, unlock, return to the app.
2. Switch to another app via the app switcher, switch back.
3. Swipe the app away and reopen it (cold start — control case).
4. Navigate to an external link and come back (bfcache restore).
5. Answer a phone call and return.

## Acceptance Criteria

- [ ] A table in the Progress Log: resume path × event, for iOS home-screen PWA.
- [ ] Same for Android Chrome, at least for paths 1, 2 and 4.
- [ ] A stated conclusion: the exact set of events `browserFreshnessTarget` must
      listen to, and which are redundant.
- [ ] Confirmation that the chosen set cannot double-fire a check on a single resume
      (or, if it can, that the debounce from task 281 absorbs it — note the required
      debounce window).
- [ ] `useFreshnessLoop.ts`'s convention comment §3 ("Three trigger points, and no
      more") is corrected to match the finding, in task 285.
- [ ] Findings recorded in PRD 017 §11 *Decided*, replacing the pending device check.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Blocked on device access;
  cannot be done by an agent session.
