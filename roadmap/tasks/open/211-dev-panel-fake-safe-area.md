# 211 — Dev panel: fake safe-area insets

**Status:** open
**Priority:** low
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] Presets in the dev panel: none / iPhone portrait / iPhone landscape
- [ ] Overrides `--sat`, `--sar`, `--sab`, `--sal` through the existing seam, not by
      touching `env()` or by editing component styles
- [ ] Does not conflict with `helpers/safeArea.ts` — verified by rotating and by
      opening the on-screen keyboard with the override active
- [ ] The bottom nav and the app bar visibly respect the faked insets
- [ ] `LayoutDebug.vue` reports the faked values, and it is clear from the panel that
      they are faked
- [ ] "None" restores live `env()` behaviour with no leftover inline properties

## Depends on

- **Task 208** — the panel this lives in.
- The safe-area work in `helpers/safeArea.ts` / `assets/main.css`.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 2.
