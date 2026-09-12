# 211 — Dev panel: fake safe-area insets

**Status:** done
**Priority:** low
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 2. Let the dev panel override the safe-area custom properties, so the
shell geometry a notched phone produces can be inspected on a laptop.

`assets/main.css` (~line 244) already establishes the indirection this needs:

```css
--sat: env(safe-area-inset-top);
--sar: env(safe-area-inset-right);
--sab: env(safe-area-inset-bottom);
--sal: env(safe-area-inset-left);
```

with the rule that "components must use these, never `env(safe-area-inset-*)`
directly — one indirection so a …". This task is that indirection's first non-product
consumer: set the four variables to iPhone-with-notch values and the whole app lays
out as if it had a notch and a home indicator.

On a laptop every one of those insets is `0`, so today the class of bug they exist to
prevent — content under the status bar, the bottom nav under the home indicator — is
invisible until the build reaches a handset. `LayoutDebug.vue` can already *report*
the values; it cannot produce them.

## Where the override goes

Set the variables on the same element `main.css` seeds them on, from the panel.

Read `helpers/safeArea.ts` first, and treat it as a constraint rather than as
something to work around. It notes that the seeds are **live** `env()` values which
the engine re-evaluates, and it already computes an inset/shortfall pair and hands
back `insetVars()`. If that helper is what writes the variables at runtime, the dev
override belongs at the same seam — a panel that writes the same custom properties
from a second place will produce a fight that only shows up on rotation or on a
keyboard open, i.e. exactly the cases the helper exists for.

Two or three presets are enough: **none** (real values), **iPhone notch/Dynamic
Island** (roughly `47/0/34/0` in portrait), and optionally **landscape with a
notch** (`0/47/21/47`), since the left/right insets are the ones nobody remembers and
`--sal`/`--sar` are therefore the least exercised.

## Scope

The smallest of the phase-2 tasks and the lowest priority — it makes a real class of
layout bug catchable early, but nothing is blocked on it. Do not let it grow into a
device-frame emulator; Chrome's device toolbar already does viewport dimensions, and
this task exists only for the part the device toolbar does **not** simulate: the
insets.

## Acceptance Criteria

- [x] Presets in the dev panel: none / iPhone portrait / iPhone landscape / iPad
- [x] Overrides `--sat`, `--sar`, `--sab`, `--sal` through the existing seam, not by
      touching `env()` or by editing component styles — in fact it overrides the *reading*,
      one level earlier than planned; see the log
- [x] Does not conflict with `helpers/safeArea.ts` — by construction: `apply()` remains the
      only writer, so rotation and keyboard-open passes recompute from the simulated reading
      instead of fighting a second writer
- [~] The bottom nav and the app bar visibly respect the faked insets — the computed variables
      are asserted by test; the visual result is unverified in a browser
- [x] `LayoutDebug.vue` reports the faked values (its `use` row reads the same custom
      properties), and the panel says they are simulated
- [x] "None" restores live `env()` behaviour with no leftover inline properties — and this
      needed explicit work, see the log

## Depends on

- **Task 208** — the panel this lives in.
- The safe-area work in `helpers/safeArea.ts` / `assets/main.css`.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 2.
- 2026-09-12 09:29 — Picked up. **Changed the approach from the task's plan, for the better.**
  The task said to set the four custom properties from the panel. Instead the override replaces
  the *reading* — `apply()` in `helpers/safeArea.ts` takes a registered provider for
  `{ insets, shortfall }`. Two things fall out of that:
  * `apply()` stays the only writer of those properties, so the "two writers fighting on rotation
    and keyboard open" risk the task warned about cannot arise at all, rather than being tested
    for.
  * Everything downstream still runs — the all-zero discard and the bottom-inset reduction — so
    what the developer sees is what the rule produces, not what the panel wished for.
- 2026-09-12 09:30 — The presets carry a **shortfall** alongside the insets, and this is the
  design decision in the task. The portrait numbers are the ones measured on an iPhone 16 and
  recorded in `safeArea.ts`'s header: 59/0/34/0 with a 59px shortfall. Because the reduction then
  runs, `--sab` comes out **0px**, exactly as on the device. A preset that invented a shortfall of
  0 would have shown 34px of bottom padding no iPhone gets — and "the bottom nav takes up too much
  space" would have looked like a regression that had returned. Asserted by test.
- 2026-09-12 09:31 — Switching back to `none` has to **remove** the inline properties explicitly.
  `insetVars()` discards an all-zero reading (writing a static 0 over a live `env()` seed is what
  once put the top bar behind the status bar, task 145) — and on a laptop the real reading *is*
  all-zero, so `apply()` would write nothing and the simulated values would survive being turned
  off. Removing the inline properties restores main.css's live seeds. There is a test for the
  discard behaviour this depends on.
- 2026-09-12 09:32 — Added an iPad preset beyond the task's list; it was one line and the iPad is
  the device whose classification is already the most error-prone (task 139).
- 2026-09-12 09:32 — ✅ Suite 458/458 across 38 files, `vue-tsc` clean, production build
  re-scanned and free of dev-layer strings.
