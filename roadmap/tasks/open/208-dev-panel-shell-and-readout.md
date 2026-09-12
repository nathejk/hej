# 208 — Dev panel: effective-state readout and device toggles

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `vue/src/components/DevPanel.vue` exists and renders only under
      `import.meta.env.DEV`
- [ ] Registered in `App.vue` next to `LayoutDebug`, and does not interfere with it
      when `SHOW_LAYOUT_DEBUG` is also on
- [ ] Collapsed handle bottom-left; expanded state persists across reloads
- [ ] While a dev profile is active, the collapsed handle shows which one
- [ ] Readout shows effective **and** real `mobile` / `standalone` / `platform`,
      clearly labelled as simulated where they differ
- [ ] Readout shows `__BUILD_ID__`, `gatesEnabled()`, served `install_gate`, and
      whether `hej.gates.bypass` is set
- [ ] Toggles for every task-206 preset (`iphone`, `ipad`, `android`, `chromium`,
      `tab`, `desktop`/`off`), applied live where possible and with a reload where not
- [ ] Uses no shadcn-vue component and no `font-nathejk`; carries the comment
      explaining why
- [ ] Displays no member data
- [ ] Absent from a production build — covered by the phase-5 bundle check, but
      verified by hand here

## Depends on

- **Task 206** — the profile it reads and writes.
- **Task 207** — the effective values it reports.

## Blocks

- **Tasks 209, 210, 211** — controls that live inside this panel.
- The phase-4 dev PIN readout.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 2.
