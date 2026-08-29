# 116 — Platform and install detection helper

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §8 (Frontend). Add `vue/src/helpers/platform.ts` — the single place the app decides
what kind of device it is running on and whether it is running installed. Everything else in
PRD 005 consumes this: the router gate, the install wall, the install instructions and the
desktop placeholder. Build it first, because it is what the rest is built on.

Three exported functions:

- `isMobileDevice()` — phone/tablet vs desktop computer.
- `isStandalone()` — is this an installed/standalone launch.
- `installPlatform()` — `'chromium' | 'ios-safari' | 'other' | 'webview'`, used to pick the
  right manual instructions (task 120).

**Pure functions with injectable `navigator`/`matchMedia`.** No module-level reads of the
globals, no cached singleton computed at import time — the dependencies come in as arguments
(defaulting to the real globals) so the unit tests can hand in a fabricated iPadOS or webview
environment without touching jsdom internals. This is not test decoration: the whole point of
this file is the awkward cases, and they cannot be exercised on the machine running the tests.

## What the signals may and may not be

**Coarse pointer + touch capability, and `navigator.userAgentData.mobile` where available.
Never viewport width** (PRD 005 §6). Width is not a device class: a phone in landscape, a
split-screen tablet and a narrow desktop window are indistinguishable by width, and the gate
this feeds decides whether a participant can reach the app at all.

Standalone detection is `matchMedia('(display-mode: standalone)')` — plus `minimal-ui` and
`fullscreen`, which are legitimate installed display modes — **OR** the iOS-only
`navigator.standalone`. Both branches are needed: iOS Safari has historically not been
trustworthy on `display-mode`, and `navigator.standalone` does not exist anywhere else.

## The tie-break, and why it is aggressive

PRD 005 §11 (decided 2026-08-30): **ambiguous devices classify as mobile.**

Detection cannot be made exact. iPadOS reports itself as macOS Safari, and touchscreen
laptops are real hardware that answers "yes" to every touch question. So the tie-break has to
be a deliberate choice rather than an accident of the boolean expression, and the asymmetry of
harms settles it:

- A false positive (desktop classified mobile) costs the user **one tap** on the "Fortsæt i
  browseren" escape hatch (task 121).
- A false negative (iPad classified desktop) leaves the user at a **placeholder page with no
  route into the app at all** — for a safety app, during an event.

Do not "improve" this into a stricter test later without reading that decision. The escape
hatch is what makes it safe; the two are a pair.

## Performance constraint

PRD 005 §6 (Non-Functional): classification is **synchronous and dependency-free**, so the
router gate can decide during `beforeEach` on a cold start without a redirect flash. That
rules out anything `await`ed — no `navigator.permissions.query`, no
`getInstalledRelatedApps`, no dynamic import. It also means this file must not import from
the stores, or the guard picks up a Pinia dependency it does not need.

Baseline is iOS/iPadOS Safari 16.4+ / Chrome 111+ per `.rules`, so guard for *absent* APIs
(`userAgentData` is Chromium-only, `navigator.standalone` is WebKit-only) but do not add
polyfills or shims for older engines.

## Acceptance Criteria

- [ ] `vue/src/helpers/platform.ts` exports `isMobileDevice()`, `isStandalone()` and
      `installPlatform()` with the return union `'chromium' | 'ios-safari' | 'other' | 'webview'`
- [ ] All three take an injectable environment (`navigator`, `matchMedia`) defaulting to the
      real globals; no module-level global access and no import-time caching
- [ ] Classification uses coarse pointer + touch capability and `navigator.userAgentData.mobile`
      where present; **no viewport-width test anywhere in the file**
- [ ] Ambiguous signals resolve to **mobile**, with the §11 reasoning stated in a comment at
      the tie-break itself, not just in this task file
- [ ] `isStandalone()` returns true for `display-mode: standalone`, `minimal-ui` and
      `fullscreen`, **or** iOS `navigator.standalone`
- [ ] Everything is synchronous; no promises, no store or config imports
- [ ] Unit tests cover, at minimum: **iPadOS reporting as macOS Safari**, a **touchscreen
      desktop**, an **in-app webview** (Facebook/Instagram), standalone via **`display-mode`**,
      and standalone via **iOS `navigator.standalone`**
- [ ] Tests also cover a plain mouse-only desktop and a plain Android Chrome phone, so the
      easy cases are pinned and cannot regress while the hard ones are being tuned
- [ ] `npm run type-check` clean

## Notes

**There is no test runner in `vue/` yet** — `vue/package.json` has no `vitest` and no `test`
script. Adding Vitest is part of this task; it is the first unit-tested module in the
frontend, and shipping the helper without the tests would defeat the reason it was designed
as pure functions. Keep the setup minimal (Vitest + a `test` script, `environment: 'node'` is
enough since the environment is injected) rather than pulling in a component-testing stack
this task does not need.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
