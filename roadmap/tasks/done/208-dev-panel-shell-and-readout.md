# 208 — Dev panel: effective-state readout and device toggles

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 2. `vue/src/components/DevPanel.vue` — the visible half of the dev
simulation layer, rendered only when `import.meta.env.DEV`, alongside `LayoutDebug`
in `App.vue` (`<LayoutDebug v-if="showLayoutDebug" />`, `App.vue:165`).

This task is the panel shell, the readout and the device-profile toggles. Tasks 209,
210 and 211 add controls into it, so the container has to be built to take them.

## The readout is the point, not the toggles

Tasks 206/207 give the app a way to *lie about the hardware*, and a silent lie is a
debugging trap: an override left on last week produces a bug the developer then
chases in application code. PRD 014 §5 names this as the layer's main failure mode
and this panel is the mitigation. So:

- When a profile is active, the **collapsed** handle must still say so — e.g.
  `SIM: iphone`. Not only the expanded panel.
- The readout shows the **effective** value *and* the real one where they differ:
  `mobile`, `standalone`, `platform`. "Simulated iOS (really: macOS, no touch)" is
  the useful line; "iOS" alone is the trap.
- Also worth having in one place: `__BUILD_ID__`, whether the gates are enabled
  (`gatesEnabled()`), the served `install_gate`, and whether `hej.gates.bypass` is
  set — the three switches that between them decide whether the app is reachable,
  currently only discoverable by reading localStorage by hand.

## Deliberately not product UI

Monospace, high-contrast, no brand font, no shadcn-vue components. This is the one
place in the repo where the `.rules` "prefer a standard shadcn-vue component"
instruction is inverted, and the reason should be in a comment in the file:
**fitting in is the anti-requirement.** A dev overlay that looks like the app will
eventually be mistaken for it in a screenshot, a demo or a bug report. In
particular, do not reach for `components/ui/drawer/` even though it would fit
mechanically.

Bottom-left, because `LayoutDebug` holds the opposite corner. Collapsed by default;
the expanded state persists (`hej.dev.*`, per task 206's prefix convention).

## Privacy

The panel shows **no member fields at all**. The only personal datum it may ever
displayed is the phone number the developer is currently typing into the login form
(the phase-4 dev PIN task). This is not a general rule applied nervously — the dev
database
holds real personal data about minors replayed from the broker (see the phpMyAdmin
note in `docker-compose.yml`), and `.rules` makes guardian phone numbers a hard
prohibition. A debug panel is exactly the surface that rule exists to pre-empt, so
it gets stated in the file rather than assumed.

## Acceptance Criteria

- [x] `vue/src/dev/DevPanel.vue` exists and renders only under `import.meta.env.DEV`
      (moved from `components/` — see the log)
- [x] Registered in `App.vue` next to `LayoutDebug`, and does not interfere with it
      when `SHOW_LAYOUT_DEBUG` is also on (opposite corners, both teleported)
- [x] Collapsed handle bottom-left; expanded state persists across reloads
- [x] While a dev profile is active, the collapsed handle shows which one, and inverts
      its colours so it is visible without reading
- [x] Readout shows effective **and** real `mobile` / `standalone` / `platform`,
      clearly labelled as simulated where they differ
- [x] Readout shows the build id, `gatesEnabled()`, served `install_gate`, and
      whether `hej.gates.bypass` is set — with a warning next to the bypass saying
      onboarding is not being tested while it is on
- [x] Toggles for every task-206 preset, applied with a reload (see the log for why a
      reload is the correct behaviour here, not a shortcut)
- [x] Uses no shadcn-vue component and no `font-nathejk`; carries the comment
      explaining why
- [x] Displays no member data
- [x] Absent from a production build — **verified against `dist/`, and it was not
      absent on the first attempt; see the log**

## Depends on

- **Task 206** — the profile it reads and writes.
- **Task 207** — the effective values it reports.

## Blocks

- **Tasks 209, 210, 211** — controls that live inside this panel.
- The phase-4 dev PIN readout.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 2.
- 2026-09-12 09:08 — Picked up. Panel written: collapsed handle, effective-vs-real readout,
  the three gate switches, preset buttons. Deliberately ugly — monospace, lime-on-black, no
  shadcn, no brand font — with the reason in a comment, since it inverts a `.rules` instruction.
- 2026-09-12 09:09 — The "real" column is obtained by calling the predicates with an explicitly
  constructed real `PlatformEnv`. That is exactly what the injectable seam is for: the panel can
  ask "what would you say without the profile?" without clearing the profile first.
- 2026-09-12 09:10 — Decision: a device change **reloads**. Not laziness — the gate decided the
  current route from the old answers, so a live switch would leave a page on screen the gate
  would no longer allow, and the app would look like it had ignored the toggle. A reload re-runs
  the chain, which is the thing being tested.
- 2026-09-12 09:12 — ⚠️ **The production build was not clean, and this is the important entry.**
  Ran `vite build` and grepped `dist/`: the fake user-agent strings and the panel's copy were
  sitting in `dist/assets/index-*.js`. Every guard was correct and every guard was useless —
  `import.meta.env.PROD`/`DEV` makes code *unreachable*, it does not remove code that something
  still statically imports. The entire safety argument of PRD 014 rested on a claim that was
  simply false, and nothing in the repo would have noticed.
- 2026-09-12 09:14 — Restructured rather than patched. New dev-only module tree `src/dev/`
  (`devDevice.ts`, `platformSim.ts`, `bootstrap.ts`, `DevPanel.vue`), reachable **only** through
  a dynamic import inside `if (import.meta.env.DEV)`. Rollup folds the branch, drops the import,
  emits no chunk. Consequences:
  * `helpers/platform.ts` no longer imports the simulation; it exposes `setDevEnvProvider()` and
    consults a *registered* provider. `permissions.ts` still goes through `devNavigator()`, so
    there is still one platform map.
  * `main.ts` uses top-level `await import('@/dev/bootstrap')` before `app.mount()`, so
    registration still precedes the router's first navigation. TLA is fine on the Safari 16.4+ /
    Chrome 111+ baseline, and in production the code does not exist at all.
  * `App.vue` renders the panel via `defineAsyncComponent`.
  * The invariant is now explicit in PRD 014 §8: **nothing in `src/dev/` may be imported
    statically from product code.**
- 2026-09-12 09:16 — `devPlatform.spec.ts` rewritten to register the provider the way
  `bootstrap.ts` does, instead of mocking the profile module. Better test: it exercises the real
  wiring (profile → simulated env → registered provider → helpers) rather than a seam that only
  exists in the test.
- 2026-09-12 09:18 — ✅ Rebuilt and re-scanned `dist/`: no `hej.dev.`, no `FBAN/FBIOS`, no
  `Pixel 8`, no panel copy, and no dev chunk emitted. Suite 444/444 across 35 files,
  `vue-tsc --noEmit` clean. Also confirmed the dev server still transforms `main.ts` with the
  dynamic import and serves `src/dev/DevPanel.vue`.
- 2026-09-12 09:19 — Task 217's bundle scan is now clearly load-bearing rather than
  belt-and-braces: this exact regression existed, was invisible, and only a scan of the artefact
  catches it. Flagged in that task.
- 2026-09-12 09:19 — Still unverified by a human in a browser: the panel's appearance, and the
  install-wall copy per simulated platform (carried over from task 207).
