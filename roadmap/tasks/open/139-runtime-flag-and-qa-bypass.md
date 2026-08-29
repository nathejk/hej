# 139 — Runtime flag and dev/QA gate bypass, plus the manual test matrix

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
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

- [ ] A runtime flag in `config/runtime.ts` disables the entire gate (device, install and
      onboarding redirects) without a redeploy
- [ ] The flag follows the existing `runtime.ts` pattern and is remembered in
      `localStorage`, so an offline start does not change the gate's behaviour
- [ ] Flag off → the app behaves exactly as it does today
- [ ] A dev/QA override bypasses the install and device gates, available as **both** a
      query param and a `localStorage` flag (the installed `start_url` drops the query)
- [ ] The override is inert in production builds/environments
- [ ] The override is the first check in the guard order (task 137)
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
