# 149 — Fix: the install wall clipped its own instructions

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

Found by the maintainer's device pass on task 139, with a screenshot from a real iPhone in Safari:
the install wall's instructions were **cut off mid-sentence** at step 4 — *"Luk Safari og åbn Hej
Nathejk fra…"* — and the note telling a Chrome-on-iOS user to switch to Safari was not visible at
all.

On the one screen whose entire purpose is those instructions, on the platform where following them
is the only way into the app.

## Cause, which is structural rather than cosmetic

Three things that are each individually correct:

1. **`main.css` sets `overflow: hidden` on `html` and `body`** — deliberately, so the whole app
   cannot be dragged (the comment there records two earlier attempts and why they were reverted).
   Scrolling in this app therefore belongs to the shell's `<main>`.
2. **`/install` and `/welcome` render *outside* the shell** (no top bar, no bottom nav — PRD 005 §7).
   So nothing above them provides a scroll container, and there is no document scrolling to fall
   back on.
3. **`Card` has `overflow-hidden`.** With `h-full` and `justify-center`, content taller than the
   viewport makes the flex children shrink, and the Card then **clips silently** instead of spilling
   visibly.

Add a real iPhone in Safari, where the viewport is ~695px because Safari's own chrome takes ~157px
of the 852px screen, and the instructions no longer fit. On the desktop and in the emulated checks
the viewport was tall enough, so it never showed up.

## Fix

Both bare routes now own their own scrolling: an `h-full overflow-y-auto` container with a
`min-h-full` inner column. `min-h-full` rather than `h-full` is what makes it behave in both
directions — **centred when it fits, top-aligned and scrollable when it does not.** Safe-area insets
moved onto the inner element, since padding on a scroll container is not part of its scrollable area
on every engine.

`/welcome` had the same defect and it matters more there, for a reason the wall does not have: **the
keyboard.** Focusing the phone number, the two digits or the guardian number shrinks the usable area
to a few hundred pixels — exactly when the member needs to see both the field they are typing in and
the button that submits it. A scroll container is also what lets the engine bring a focused input
into view at all.

## Acceptance Criteria

- [x] The install wall's instructions are fully reachable at a 695px viewport
- [x] `/welcome` scrolls, so a step taller than the viewport (or one with the keyboard open) can be
      completed
- [x] Content is still vertically centred when it does fit
- [x] Safe-area insets still applied, and inside the scrollable area
- [x] A regression guard exists, with its limitation stated
- [x] `vue-tsc`, `npm test`, `npm run build` clean

## Progress Log

- 2026-08-31 — Reported from the device pass. **Reproduced headlessly before changing anything**:
  Chrome at `--window-size=393,695` with an iPhone UA produced the identical clipping, which turned a
  device-only report into a two-second local check.
- 2026-08-31 — Fixed both views. Verified by screenshot at the same viewport: step 4 now reads in
  full and the Chrome/Firefox note — previously invisible — is there, with the content continuing
  below the fold as scrollable rather than truncated.
- 2026-08-31 — **The regression guard asserts the mechanism, not the symptom**, and that is a
  deliberate compromise worth recording: the clipped text *is in the DOM* and merely unpainted, so no
  DOM assertion can see this bug. `check-install-gate.sh` therefore checks that the wall provides a
  scroll container at all, which is the invariant that was missing, and the script names
  `--screenshot --window-size=393,695` as the way to reproduce the visual symptom.

  A screenshot comparison would catch it directly but needs a baseline image and a human to arbitrate
  every intentional change, which is a maintenance cost this repo has not chosen to take on.
- 2026-08-31 — Worth naming the general lesson, because there are two more routes that could grow
  into it: **any route rendered outside the shell has no scrolling unless it brings its own**, and
  `Card`'s `overflow-hidden` means the failure is silent rather than ugly. The two that exist are now
  fixed; a third should copy the pattern rather than rediscover this.
