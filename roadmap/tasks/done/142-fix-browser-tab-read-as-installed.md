# 142 — Fix: a browser tab could read as installed and skip the wall

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

Reported by the maintainer, 2026-08-30: on a mobile device, `hej.nathejk.dk` went to
`/welcome` and started onboarding at "step 1 af 4" — **in an ordinary browser tab**. The
intended behaviour, restated by the maintainer and already in PRD 005 §5/§6, is:

- **Browser on a phone/tablet → the install instructions, directly.** Nothing else.
- **Only a home-screen (standalone) launch** runs the flow: login → confirm profile if not
  already verified → location → push, each skipped if already settled.

The gate only lets a mobile visitor past the install wall if `isStandalone()` is true or the
"Fortsæt i browseren" escape hatch has been taken, so a tab reaching `/welcome` means one of
those two answered wrongly.

## The defect

`isStandalone()` accepted three display modes as "installed":

```ts
const INSTALLED_DISPLAY_MODES = ['standalone', 'minimal-ui', 'fullscreen']
```

The comment justified it as "`minimal-ui` and `fullscreen` are as installed as `standalone` —
a manifest may legitimately ask for either". True in general, and **irrelevant here**: this
app's manifest asks for `display: 'standalone'` and nothing else (`vite.config.ts`). So those
two modes can never occur on an installed launch of *this* app — they can only occur on an
uninstalled one, which makes them pure false positives:

- **`fullscreen` matches whenever the browsing context is fullscreen.** Play a video
  fullscreen in a tab and the tab starts claiming to be an installed app.
- **`minimal-ui` is matched by several mobile browsers** for their own chrome-reduced reading
  modes (Samsung Internet, Firefox for Android), in a plain tab.

A tab that reads as installed skips the wall and lands in onboarding — the exact configuration
PRD 005 exists to prevent, and silently: on iOS that user can never receive Web Push, and
nothing in the UI would ever say so.

## The fix

Three checks, in order, with the middle one new:

1. iOS's `navigator.standalone` — WebKit-only, true only in a home-screen app, and the one
   unambiguous signal on the platform where installing matters most.
2. **An explicit `display-mode: browser` veto.** Only a real tab reports `browser`, so when the
   engine says so, nothing else may override it.
3. `display-mode: standalone` — the mode the manifest actually requests.

Erring towards "not installed" is the safe direction: the cost is showing the wall to someone
already installed, who taps "jeg har allerede installeret appen"; the cost of the opposite is a
participant spending the event in a tab with no notifications, unaware.

## Acceptance Criteria

- [x] `minimal-ui` and `fullscreen` no longer count as installed
- [x] An explicit `display-mode: browser` match vetoes everything except iOS's
      `navigator.standalone`
- [x] iOS `navigator.standalone` still wins, including against a stray `browser` match
- [x] Tests cover both former false positives (fullscreen video, chrome-less mobile browser)
- [x] PRD 005 §6's detection requirement corrected, since it specified the broken rule
- [x] `vue-tsc`, `npm test` and `npm run build` clean

## What this does not explain

The other way a tab can get past the wall is a **stuck `hej.install.continue-in-browser`**:
the escape hatch is persisted per browser with no expiry (PRD 005 §12 asks whether it should
have one and deliberately leaves it open). Anyone who tapped "Fortsæt i browseren" while
testing has it set, and it is invisible from the app afterwards.

The documented way back is the **"App på hjemmeskærmen" row on Min profil** (task 121), whose
action clears the override and returns to the wall. Clearing site data works too. If the
symptom persists on a device where that override was never tapped, the next suspects are a
stale service worker serving an older bundle (hard-reload / clear site data), and then
`?nogate=1` (non-prod) to establish whether the gate is involved at all.

## Progress Log

- 2026-08-30 — Reported: mobile browser reaches `/welcome` instead of the install wall.
- 2026-08-30 — Traced by elimination rather than by guesswork: the gate has exactly two ways to
  let a mobile visitor past the wall, so the fault had to be in `isStandalone()` or the
  persisted escape hatch. Reviewing `isStandalone()` against the actual manifest showed the
  display-mode list accepted two modes this app is never launched in.
- 2026-08-30 — Fixed, with the `browser` veto added rather than merely narrowing the list: a
  positive "this is a tab" signal is stronger than the absence of a positive "this is installed"
  one, and it also covers whatever the next engine does oddly.
- 2026-08-30 — Three tests added for the two false positives and the veto; the old test that
  asserted "minimal-ui and fullscreen are equally installed" was **inverted**, since it encoded
  the bug. 37 tests pass, `vue-tsc` and `npm run build` clean.
- 2026-08-30 — Recorded above what this fix does *not* rule out, so the next report can start
  from the escape hatch and the service worker rather than re-deriving that list.
