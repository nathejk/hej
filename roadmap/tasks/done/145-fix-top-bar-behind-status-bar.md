# 145 — Fix: the top bar rendered behind the status bar

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

Reported by the maintainer, 2026-08-30, with a screenshot from the installed app on iPhone: the
top bar's wordmark and the user avatar were drawn **behind the status bar**, colliding with the
clock and the battery indicator.

The `LayoutDebug` overlay in that screenshot is what made this quick, and is worth the credit:

```
sa   59/0/34/0      ← raw env(safe-area-inset-*)
use  0/0/0/0        ← our --sat/--sar/--sab/--sal
main.top 69
```

So the platform was reporting a 59px top inset while the app was applying zero, and
`main.top 69` corroborates it exactly: header padding (12px) + content (~44px) + padding (12px)
+ border (1px) with **no inset at all**.

## Cause

`readInsets()` resolves `env()` on a throwaway element and `apply()` writes the numbers to
`--sat`…`--sal` on the root. Two facts collide:

1. **The seeds in `main.css` are live.** `--sat: env(safe-area-inset-top)` is re-substituted by
   the engine whenever the insets change. What `apply()` writes is a **static pixel value that
   shadows the seed.**
2. **On iOS standalone the insets read as 0 until the first frame is painted**, and
   `initSafeArea()` runs *before mount* — deliberately, so the bottom correction lands before
   first paint rather than reflowing an already-measured scroll container.

So on a cold launch the app wrote `--sat: 0px` over a seed that would have resolved to 59px
moments later, and nothing ever corrected it: a normal launch is followed by no resize, no
orientation change and no visibility change, which are the only triggers for a re-read.

The bug is therefore not "the number was wrong" but **"we replaced a self-correcting
declaration with a frozen wrong one"** — writing a value we are not sure about is strictly worse
than writing nothing.

## Fix

- **An all-zero reading is discarded.** It is exactly the signature of "not resolved yet", and
  it is also the state of a device with genuinely no insets (Android, desktop) — where leaving
  the seed alone is equally correct, because the seed resolves to 0 too. So discarding it costs
  nothing, and it is the only reading that carries no information the CSS does not already have.
- **A second pass on the next frame**, so a late-resolving inset is picked up. The synchronous
  pass is kept: where the insets *are* already available it still applies the bottom correction
  before first paint, which is what avoids the reflow this file's header warns about. The second
  pass changes nothing on a device that answered immediately, so it cannot reintroduce that.
- The decision is split into a pure `insetVars(inset, shortfall)` so it can be tested without a
  DOM — the *reading* needs a browser, the *rule* does not.

## Acceptance Criteria

- [x] An all-zero inset reading leaves the CSS seeds untouched
- [x] A reading with any non-zero edge is applied as before, top passed through unchanged
- [x] The bottom inset is still reduced by the viewport shortfall, still clamped at zero
- [x] A late-resolving inset is picked up without waiting for a resize
- [x] The synchronous first pass is preserved for devices that answer immediately
- [x] Tests cover the all-zero rule and were confirmed to fail without the fix
- [x] `vue-tsc`, `npm test` and `npm run build` clean

## Progress Log

- 2026-08-30 — Reported with a screenshot. Diagnosed from the `LayoutDebug` overlay in it rather
  than by reasoning about the layout: `sa 59` against `use 0` says the platform and the app
  disagreed, which narrows it to `apply()` immediately, and `main.top 69` confirmed the inset was
  absent rather than merely small. This is the second time in two days that overlay has turned a
  guessing game into a five-minute read (see task 142); it earns its keep.
- 2026-08-30 — Fixed by discarding an uninformative reading rather than by moving the call after
  mount. Moving it would have traded this bug for the reflow the file's header records having
  already been reverted once — and the *root* problem is that a static write shadows a live seed,
  which no amount of retiming addresses.
- 2026-08-30 — **Verified the test fails without the fix**: removing the guard fails "writes
  nothing for an all-zero reading" and nothing else. 46 tests pass with it in place.
- 2026-08-30 — ✅ `vue-tsc`, 46 tests, `npm run build` clean.

  Not verifiable from here: that the header now clears the status bar on the device. The overlay
  is the check — `use` should read `59/0/0/0` on that iPhone (top passed through, bottom reduced
  from 34 by the 59px shortfall), and `main.top` should be 128 rather than 69.

  Honest limitation: I could not establish *why* this surfaced in this release rather than
  earlier. The mechanism above does not depend on anything PRD 005 changed, and the pre-mount
  read has always been able to return zeros — so my best reading is that it was latent and
  timing-dependent, and something about the current bundle made the unresolved read reliable
  rather than intermittent. If it recurs, the overlay's `use` line is the first thing to look at.
