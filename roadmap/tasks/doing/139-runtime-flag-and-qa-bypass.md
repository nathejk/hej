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
      progress log — including misclassifications found, not only passes
- [ ] iPadOS is confirmed to classify as **mobile**
- [ ] The in-app-webview case reaches the escape hatch and gets into the app
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
  | iOS Safari | add-to-home-screen, `navigator.standalone`, push on 16.4+ | not run |
  | Android Chrome | `beforeinstallprompt` captured **before mount**, `appinstalled` seen | not run |
  | Android Firefox | no prompt → manual instructions correct and readable | not run |
  | Desktop | classified desktop, leaves the app for `/desktop.html`, never sees `/welcome` or `/install`, and the install banner stays **hidden** | not run |
  | iPadOS | reports as macOS Safari — must still classify as **mobile**; if it does not, the placeholder's install banner must be visible as the way back | not run |
  | In-app webview (Facebook) | install impossible; escape hatch reachable and sufficient | not run |

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
  banner appearing on a tablet but not on a desktop (task 140).
