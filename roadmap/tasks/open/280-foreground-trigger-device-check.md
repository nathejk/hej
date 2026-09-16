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

A diagnostic page now does the recording: **`/genoptag`**, reached from `/sporing`
("Se genoptagelses-diagnostik"), which is itself linked from the privacy page. Unlisted,
like `/sporing` — a measurement tool, not a feature.

On an **installed home-screen PWA** on iOS/iPadOS (baseline: Safari 16.4+), and on Android
Chrome for comparison:

1. Open `/genoptag`.
2. Pick the path you are about to test from the row of buttons.
3. Leave the app that way, and come back.
4. Read the verdict for that group. Repeat for each path.
5. Tap **Kopiér** and paste the Markdown into this log.

The page records `pagehide`, `pageshow` (with `persisted`), `freeze`, `resume`,
`visibilitychange`, `focus`, `blur`, `online` and `offline`, each with
`document.visibilityState`, and marks per row whether the freshness loop would have
**acted** on it — `visibilitychange` counts only when the document became *visible*, since
the loop stops when hidden. The log is persisted to `localStorage` on every event, because
iOS may kill the app while it is hidden and that is precisely one of the paths under test.

The paths, as offered on the page:

1. Lock the screen, unlock, return to the app.
2. Switch to another app via the app switcher, switch back.
3. Follow the external link on the page and come back (bfcache restore).
4. Answer a phone call and return.
5. Swipe the app away and reopen it — **the control**. A cold start must show up as one, or
   the log cannot be trusted for the other four.

**What a failure looks like:** a group whose verdict reads "hændelser, men INTET tjek". That
is the finding that would make PRD 017 silently stale on that resume path, and the reason
the page shows a verdict rather than only rows.

## Acceptance Criteria

- [ ] A table in the Progress Log: resume path × event, for iOS home-screen PWA.
      (Paste the probe's **Kopiér** output.)
- [ ] Same for Android Chrome, at least for paths 1, 2 and 4.
- [ ] A stated conclusion: the exact set of events `browserFreshnessTarget` must
      listen to, and which are redundant.
- [ ] Confirmation that the chosen set cannot double-fire a check on a single resume
      (or, if it can, that the debounce from task 281 absorbs it — note the required
      debounce window).
- [ ] `useFreshnessLoop.ts`'s convention §5 ("Four trigger points, and no more") and
      `LOOP_EVENTS` in `helpers/resumeProbe.ts` both updated to match the finding.
- [ ] Findings recorded in PRD 017 §11 *Decided*, replacing the pending device check.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Blocked on device access;
  cannot be done by an agent session.
- 2026-09-16 17:30 — **Probe built** so the device check is a two-minute job:
  `vue/src/views/ResumeProbeView.vue` at `/genoptag`, with the rules in
  `vue/src/helpers/resumeProbe.ts` and 12 tests in `resumeProbe.spec.ts`.
  Notes on the design:
  - **The rules live in a helper, not the view**, so the "would the loop have acted on this?"
    logic is tested without a DOM. A page that rendered a plausible table and got that column
    wrong would be worse than no page — it would produce a confident wrong answer that then
    gets quoted in this log for the rest of the project.
  - **`LOOP_EVENTS` is deliberately duplicated** from `browserFreshnessTarget` rather than
    imported. It is the claim under test: a probe that tracked the implementation could never
    disagree with it, and disagreeing is its entire job. If the loop's listeners change, this
    list is updated as a decision.
  - **Written to `localStorage` inside each handler.** Not an optimisation: iOS may kill the
    app while hidden, and `pagehide` may be the last code that runs. An in-memory log would
    lose the evidence for the exact case this exists to catch. Capped at 400 entries so a page
    left open cannot eat the quota the offline map needs.
  - **Grouped by the tester's chosen path, with a verdict per group**, and revisiting a path
    starts a new group rather than merging — a merged verdict could hide a failure inside a
    success.
  - **No new config flag.** Follows `/sporing`'s precedent (task 082): unlisted route, linked
    from the other field diagnostic. That also means it works in a normal production build,
    which matters, because the whole question is about the *installed* app.
  - `npm run build` verified: it lands as its own lazy chunk.
- Still needs a phone. Everything above is scaffolding for the measurement, not the
  measurement.
- 2026-09-16 23:15 — **First real use on an installed iOS PWA, and it caught two bugs in the
  probe rather than in the app.** iOS 18.7, Safari 26.6.1, `standalone=true`,
  `iosStandalone=true` — so the setup was valid. The log contained one row:

  | tid | gap | event | visibility | persisted | loop? |
  |---|---|---|---|---|---|
  | 23.13.53 |  | `pageshow` | visible | false | nej |

  filed under **lås / lås op** with the verdict "hændelser, men INTET tjek". That reads exactly
  like the damning finding this page exists to catch, and it means nothing of the kind — it is the
  page opening. **No resume was measured.** Two defects, both mine:

  1. **The mount was recorded as a synthetic `pageshow`.** So it is indistinguishable from a real
     `pageshow` fired by iOS on resume; a log containing both could not say which was which,
     corrupting the one measurement the page exists to take. Now recorded as its own `mount`
     pseudo-event.
  2. **`mount` was not in `LOOP_EVENTS`, so it reported `loop? nej` — which is wrong.**
     `useFreshnessLoop` checks on construction when the document is visible ("mounting counts as
     foregrounding", PRD 017 §6). The consequence was worse than a wrong cell: the **cold-start
     control** would have reported itself as a failure, and the control is what tells you whether
     to trust the other four rows.

  Plus the cause of the false label: the path selector **defaulted to the first path**, so merely
  opening the page filed its mount under "lås / lås op" and invented a failed lock/unlock test
  nobody had run. It now starts on "ingen vej valgt", says so in amber, and a group containing
  only mounts gets the neutral verdict "siden blev åbnet — ingen genoptagelse målt endnu".

  A diagnostic that cries wolf on first sight is worse than no diagnostic — exactly the
  confident-wrong-answer failure this task's own log claimed to be guarding against.
- 2026-09-16 23:20 — One genuine data point survives from that run: on a **cold start** in an
  installed iOS 18.7 PWA, `pageshow` fires with `persisted=false` while already `visible`, and no
  `visibilitychange` accompanies it. Consistent with the loop's mount-time check being the only
  thing that refreshes a cold start — which it does. Not evidence about any of the five resume
  paths.
