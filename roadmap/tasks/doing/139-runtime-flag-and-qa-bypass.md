# 139 — Runtime flag and dev/QA gate bypass, plus the manual test matrix

**Status:** doing
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:**

## Description

PRD 005 §6 and §10. Three things that belong together because they are all about being
able to survive the gate being wrong: a kill switch, a bypass, and the manual testing that
finds out where it is wrong.

## The runtime flag

Ship the **whole gate** behind a runtime flag in `vue/src/config/runtime.ts` (served from
`GET /api/config`, `go/cmd/api/config.go`) so it can be disabled without a rollback if it
misfires **during an event**.

This is not caution for its own sake. The failure mode is *participants unable to reach a
safety app* — an over-eager device classification or an unreliable `display-mode` in some
webview and a member standing in a forest cannot get to the map, the SOS page or their
contacts. A redeploy is not an acceptable response time for that, so a kill switch is not
optional. Follow the existing pattern in `runtime.ts`, including remembering the value in
`localStorage`, since an offline start must not silently flip the gate's behaviour (the
same reason the diagnostic flags are persisted there).

## The dev/QA override

A query param **or** `localStorage` flag, **gated to non-prod**, that bypasses the install
and device gates. It is the first check in the guard order (task 137) so everything after
it can be exercised in a normal browser tab. Note the constraint already recorded in
`runtime.ts` about `?debug=`: the manifest's `start_url` is `/`, so an installed launch
drops the query string — which is precisely why a `localStorage` form is needed too.

This override is **not** the answer to "an organizer needs laptop access" (PRD 005 §11) —
that would be a new PRD.

## The manual test matrix

Detection is heuristic and PRD 005 §8 mitigates that **with the escape hatch rather than
with better sniffing**. That makes this matrix the mechanism by which we find out where the
heuristics are wrong; it cannot be automated from here, and no unit test substitutes for it.

| Platform | What to establish |
|---|---|
| iOS Safari | add-to-home-screen flow, `navigator.standalone` detection, push on 16.4+ |
| Android Chrome | `beforeinstallprompt` captured before mount, `appinstalled` observed |
| Android Firefox | no `beforeinstallprompt` → manual instructions are correct and readable |
| Desktop | classified desktop, lands on `/desktop`, never sees `/welcome` or `/install` |
| iPadOS | reports itself as **macOS Safari** — verify it still classifies as **mobile** (PRD 005 §11: ambiguous → mobile) |
| In-app webview (e.g. Facebook browser) | installation is impossible; the escape hatch must be reachable and sufficient |
| **Legacy device below the baseline** | **added 2026-09-02.** A device too old to run the bundle never reaches the gate at all, so this row is not about classification — it is about what is on screen when no JavaScript of ours has run. Tested on an iPad mini 2 / iOS 12.5.8: it showed a **blank white page** until task 204 added a static fallback. |

The iPadOS row is the one most likely to fail, and the tie-break exists because of it: a
false positive costs a desktop user one click on "Fortsæt i browseren", a false negative
leaves an iPad user at a placeholder with no route into the app at all.

The webview row is a pass/fail on the escape hatch, not on installation.

## Acceptance Criteria

- [x] A runtime flag in `config/runtime.ts` disables the entire gate (device, install and
      onboarding redirects) without a redeploy
- [x] The flag follows the existing `runtime.ts` pattern and is remembered in
      `localStorage`, so an offline start does not change the gate's behaviour
- [x] Flag off → the app behaves exactly as it does today
- [x] A dev/QA override bypasses the install and device gates, available as **both** a
      query param and a `localStorage` flag (the installed `start_url` drops the query)
- [x] The override is inert in production builds/environments
- [x] The override is the first check in the guard order (task 137)
- [ ] The manual matrix above is executed and the outcome per row recorded in this task's
      progress log — including misclassifications found, not only passes. **Three of seven rows are
      now automated and passing; the four device rows are outstanding.**
- [x] iPadOS is confirmed to classify as **mobile** — **verified on hardware 2026-09-02.** See the log:
      it classified as mobile in the browser (install wall shown) and as standalone once installed.
- [~] ~~The in-app-webview case reaches the escape hatch and gets into the app~~ — **superseded by
      task 143**: there is no escape hatch, and no login outside the installed app. What replaces it
      is verified automatically for the iOS webview (told to reopen in Safari, and *not* shown
      add-to-home-screen steps). That the advice is followable on a real phone is still unrun.
- [ ] Any misclassification found is either fixed or explicitly accepted with the escape
      hatch named as the mitigation

## Depends on

- **Task 137** — the guard this flag disables and this override bypasses.
- **Task 116** — the platform helper whose heuristics the matrix is testing.
- **Task 117** — the install store, for `beforeinstallprompt` / `appinstalled` /
  `continueInBrowser`.
- **Task 123** — `DesktopView`, for the desktop and iPadOS rows.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — Picked up. **The code half is complete; the matrix is not, and cannot be from here —
  see the last entry.** Left in `doing/` rather than closed, because closing it would claim a device
  pass that has not happened.
- 2026-08-30 — **The runtime flag, end to end.** `INSTALL_GATE` env var / `-install-gate` flag →
  `config.installGate` → `install_gate` on `GET /api/config` → `installGateEnabled` in
  `config/runtime.ts` → `gatesEnabled()` in `config/gates.ts`, which task 137's guard already
  consults. Setting `INSTALL_GATE=false` disables device classification, the install wall redirect
  and the onboarding redirect together, with no redeploy.

  Three decisions, all about which way a missing value should fail:

  - **Defaults ON everywhere, including development.** A gate that is off by default is a gate
    nobody tests, and the installed app is the only supported way to use this.
  - **An absent `install_gate` in the response means ON**, not off. An older BFF that does not send
    the field must not silently disable the gate.
  - **A cold start with nothing remembered means ON**, which needed a second helper
    (`rememberedFlagOr`) rather than reusing `rememberedFlag`: "never heard from the server" and
    "the server said off" must not collapse into the same answer for a switch that decides whether
    the app is reachable at all. The diagnostic flags can safely default false; this one cannot,
    because a first offline start would otherwise skip onboarding entirely — and nothing else in the
    app ever asks for location or notifications.

  The value is remembered in `localStorage` like the diagnostics, so if an organizer switches the
  gate off mid-event, a member whose phone starts without coverage still gets the ungated app rather
  than the wall.
- 2026-08-30 — **Dev/QA override:** `?nogate=1` persists `hej.gates.bypass`, and the *flag* is what
  `gatesEnabled()` consults. Both forms are required for a structural reason, not convenience: the
  manifest's `start_url` is `/`, so an installed launch drops the query string — the query form is
  unavailable in precisely the mode the gate exists to produce (the same constraint `runtime.ts`
  already records for `?debug=`). `?nogate=0` clears it, because an override with no off switch is
  one that gets left on. Guarded by `import.meta.env.PROD`, so Vite compiles the branch out of a
  production bundle — a URL cannot talk production into the bypass. Read once in `main.ts` before
  the router's first navigation, so the first gate check already sees it.
- 2026-08-30 — Two Go tests pin the flag's default and its switchability. The default is the part
  worth a test: a kill switch that quietly defaults to "off" is one nobody notices until a member
  arrives at an event in a browser tab with no notifications.
- 2026-08-30 — **The manual matrix has NOT been executed.** There is no browser and no device in
  this environment, so every row of it is unverified. This is not a formality to be waved through:
  PRD 005 §8 mitigates heuristic detection *with the escape hatch rather than with better sniffing*,
  which makes this matrix the only mechanism by which we find out where the heuristics are wrong.
  What needs a human with the hardware:

  | Platform | What to establish | State |
  |---|---|---|
  | Desktop | classified desktop, leaves the app for `/desktop.html`, never sees `/welcome` or `/install`, install banner stays **hidden** | **PASS — automated**, `vue/scripts/check-install-gate.sh` |
  | iPhone browser tab | lands on the wall, iOS add-to-home-screen instructions, no one-tap button, **no login anywhere**, link out to the website | **PASS — automated**, and **confirmed on a real iPhone** 2026-08-31 |
  | In-app webview (iOS) | told to reopen in a real browser, and **not** shown add-to-home-screen steps | **PASS — automated** |
  | iOS Safari, installed | add-to-home-screen actually works, `navigator.standalone`, Web Push on 16.4+, safe-area (`use 59/0/0/0`, `main.top 128`) | not run — needs an iPhone. Note the pre-fix state *was* observed: `sa 59/0/34/0` against `use 0/0/0/0` is what task 145 fixed |
  | Android Chrome | `beforeinstallprompt` captured **before mount**, `appinstalled` seen, one-tap install | not run — needs an Android device (see below) |
  | Android Firefox | no prompt → the browser-menu instructions are correct and readable | not run — needs an Android device |
  | iPadOS | reports as macOS Safari — must still classify as **mobile** | not run — needs an iPad |

  Two rows are worth more attention than the rest. **iPadOS** is the one most likely to fail and the
  reason the tie-break exists (`platform.ts` detects it as `platform === 'MacIntel' && maxTouchPoints > 1`,
  which is a heuristic about hardware, not a supported API). **Android Chrome** is the only way to
  confirm the `beforeinstallprompt` timing: if the listener is registered too late the failure is
  *silent* — the wall simply shows manual instructions instead of the one-tap button, and it will not
  reproduce reliably in dev.

  Also unverified for the same reason, carried over from the tasks that deferred them here:
  light/dark rendering and keyboard operability of the new `checkbox` (task 122), the full-screen
  `PermissionPrompt` variant (task 130), grant/deny on both permission steps (task 131), that
  `UpdatePrompt` no longer overlays the wall (task 138), and the desktop placeholder's install
  banner appearing on a tablet but not on a desktop (task 140), and the guardian-correction field
  (task 148: number keypad shows, and the browser does not offer the member's own number as an
  autocomplete suggestion), and the portrait nudge banner (task 146: renders in the shell without
  covering content, and the capture sheet opens from it).
- 2026-08-31 — **Automated the part of the matrix a real browser can decide**, in
  `vue/scripts/check-install-gate.sh`: host Chrome headless against the dev stack, throwaway profile
  per case (so no leftover session, service worker or localStorage can invalidate an assertion),
  asserting both presence and absence. 15 checks across three cases, all passing.

  This does not replace the matrix, and the task stays in `doing/`. What it replaces is the part
  that was being re-checked by hand on a laptop every time the gate changed — which is the part that
  broke twice in one week (tasks 141 and 142) and would have been caught here in seconds. The
  absence assertions are the valuable half: "no login form anywhere in a browser tab" is task 143's
  central rule, and it is now checked on three different user agents rather than believed.

- 2026-08-31 — **A finding from the attempt, worth more than the checks it cost.** The Android case
  failed, and the app was right while the test was wrong: headless Chrome with an Android UA is still
  a **mouse-only, touch-less desktop**, so it classified as desktop and went to the website.

  `isMobileDevice()` treats only the *Apple* UA patterns as decisive; for everything else it requires
  touch or pointer evidence, deliberately — `navigator.userAgentData.mobile` is false on an Android
  *tablet*, and a UA string is not evidence of hardware. So the iPhone rows are representable here
  only because `isAppleTouchDevice()` short-circuits on the UA before touch is consulted, and
  **Android and iPadOS cannot be represented by any UA string.**

  Deliberately *not* asserted as expected behaviour: pinning "Android UA → desktop" would enshrine
  something we do not want, and would fail the day someone sensibly makes the UA decisive for Android
  too. It is documented in the script's header instead, with the consequence stated plainly —
  **Android must be tested on Android**, and the iPadOS row is unreachable from any laptop.

- 2026-08-31 — Still outstanding, and needing hardware rather than effort: the four device rows
  above, plus the visual checks deferred here from tasks 122, 130, 131, 138, 140, 145, 146 and 148.
  The fastest useful subset, if time is short: **iPadOS classification** (one screenshot of the
  `LayoutDebug` overlay tells you), **Android Chrome one-tap install**, and **safe-area on the
  iPhone** (`use` should read `59/0/0/0`, `main.top` 128).

- 2026-08-31 — **First real-device evidence, from the maintainer: the iPhone-browser row passes.**
  The wall renders on real iOS Safari with the correct iOS instructions, the website link and no
  login — matching the automated checks, which is the useful part: it says the UA-driven emulation is
  representative for that row rather than merely self-consistent.

  Also confirmed incidentally: `sa 0/0/0/0` in a Safari *tab* is correct, not a repeat of task 145.
  Safari's own chrome absorbs the notch there (`vp 695` against `scr 852`), so there are genuinely no
  insets to apply — and task 145's rule of discarding an all-zero reading leaves the live CSS seed in
  place, which resolves to the same zero. The row that still needs checking is the *installed* one,
  where the insets are real (`use` should read `59/0/0/0`, `main.top` 128).

- 2026-08-31 — **The device pass immediately earned its keep: it found task 149.** The wall was
  clipping its own instructions at step 4 on a 695px viewport — visible on hardware, invisible on a
  desktop and invisible to every automated check, because the clipped text is present in the DOM and
  merely unpainted. Fixed, and reproducible locally now that the viewport size is known.

  Worth noting what that says about this matrix: the rows that remain are not a formality. Three of
  the six problems found this week (142, 145, 149) were only observable on a real phone.

- 2026-09-02 — **iPadOS row run on hardware, against the deployed host `https://hej.nathejk.dk`.**
  Device: **iPad (6th generation), iPadOS 17.7.10, MR7F2KN/A** — a Wi-Fi-only model, and one that is end
  of life at iPadOS 17. Worth recording precisely, because two later findings turn on it: it is inside the
  16.4+ baseline, and it has **no GPS receiver** (see tasks 197, 198). This is
  the row the matrix called most likely to fail, and it passed on both counts:

  | check | result |
  |---|---|
  | classified **mobile** in Safari despite the macOS user agent | pass — the install wall was shown, debug overlay `browser / install` |
  | add-to-home-screen instructions correct for iPadOS Safari | pass — the Del/Share sheet contained `Føj til hjemmeskærm` as described |
  | classified **standalone** once opened from the home screen | pass — debug overlay `standalone / maps`, `standalone / profile`, app chrome present |
  | reached `/welcome` and then the app, never stuck at a placeholder | pass — signed in, profile and map both rendered |

  So the tie-break rule from PRD 005 §11 (ambiguous → mobile) is doing its job on the device it was
  written for, and the false-negative risk it was hedging against — an iPad user stranded at a
  placeholder with no way in, which task 143 made unrecoverable by removing the escape hatch — did not
  materialise.

- 2026-09-02 — **One failure found, and it is not a classification bug: location could not be enabled.**
  Tapping "Slå placering til" on the map did nothing at all — no iOS dialog, no error, no state change,
  and the profile page went on reporting `Placering — Fra`. Filed as **task 197**, which turned out to be
  a fault in our code regardless of what WebKit did: every failure path was silent, including a
  `getCurrentPosition` that never calls either callback (the API's own `timeout` bounds acquiring a fix,
  not waiting for the dialog). Fixed there, with a wall-clock guard, a pending state on the button, and a
  named cause per error code. **The iPadOS row above is unaffected** — this is a map/permission bug, not a
  device-detection one.

- 2026-09-02 — Incidental but useful: the readiness section reported `6,4 MB gemt · 0 % af telefonens
  plads`. Rounding to zero at 6.4 MB puts the origin quota in the gigabytes, which is the first real
  evidence about the iOS 26 storage policy — PRD 009 §8's ~1 GB planning ceiling is conservative on a
  current iPad. Recorded in PRD 009 rather than acted on: the ceiling exists for iOS 16.x devices, and
  whether any participant still has one is a fleet question.

- 2026-09-02 — **Remaining rows: Android Chrome, Android Firefox, and installed iOS Safari on a phone.**
  Three of seven are automated, iPadOS is now done on hardware, so three device rows are left.

- 2026-09-02 — **A row nobody had thought of, found by testing an old device: below the baseline, the gate
  does not run at all.** An iPad mini 2 on iOS 12.5.8 showed a blank white page — not the desktop
  placeholder, not an unsupported message. Everything in this matrix assumes the app boots well enough to
  classify the browser, and the row above now records the case where it does not. Fixed in task 204 with
  static markup in `index.html`; the matrix keeps the row because the fallback is the sort of thing a future
  build tool change could silently drop.
