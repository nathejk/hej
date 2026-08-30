// Safe-area custom properties.
//
// Components must use `var(--sat)` / `var(--sar)` / `var(--sab)` / `var(--sal)` rather
// than `env(safe-area-inset-*)` directly. main.css seeds them, so the CSS-only result is
// correct before this runs; the indirection is what lets the one correction below live in
// a single place instead of at a dozen call sites.
//
// THE TOP INSET IS PASSED THROUGH UNCHANGED, and that is the conclusion of a wrong turn
// worth recording. Measured on an iPhone 16 standalone: screen 393x852, innerHeight 793,
// sa.top 59. That looks like a double-counted inset, and an earlier version zeroed `--sat`
// accordingly — but `main.getBoundingClientRect().top` was 0 with the map visibly drawing
// behind the status bar, which proves nothing in our layout reserves it. Zeroing it would
// have pulled the map controls up under the clock.
//
// A second attempt added the 59px shortfall to the shell's height, to close the strip of
// background below the nav. Also reverted: it gave the document scrollable overflow so the
// whole shell could be dragged, and resizing the shell after first paint left the scroll
// container in `main` mis-measured on a cold open. See the note in main.css.
//
// THE BOTTOM INSET IS REDUCED BY THE VIEWPORT'S SHORTFALL. This is geometry, not an
// inference about platform behaviour:
//
//   * `env(safe-area-inset-bottom)` exists to keep content clear of the home indicator at
//     the physical bottom of the screen.
//   * iOS clips painting to the reported viewport, which ends `shortfall` pixels above the
//     screen's bottom edge — nothing we lay out can reach past it (see main.css for the two
//     failed attempts to make the shell reach the bottom).
//   * So the shell already ends 59px above the screen edge, comfortably clear of a 34px
//     indicator, and the nav's own 34px of padding is redundant. That redundancy is what
//     "the bottom nav takes up too much space" is.
//
// The padding still needed is therefore `max(0, inset.bottom - shortfall)`. It can only
// ever *reduce* padding that is provably unnecessary: when the viewport does fill the screen
// the shortfall is 0 and the full inset survives, and Android reports no bottom inset so it
// is 0 either way. The shortfall is clamped at 0 because desktop reports a viewport taller
// than `screen.height`.
//
// This does NOT close the strip of backdrop below the viewport — nothing can. It removes the
// 34px we were adding on top of it.

const EDGES = ['top', 'right', 'bottom', 'left'] as const

function readInsets(): Record<(typeof EDGES)[number], number> {
  // `env()` is only valid in CSS values and a custom property referencing it reads back
  // unsubstituted, so the only way to get the numbers is to let the engine resolve them
  // on a throwaway element and read the computed result.
  const probe = document.createElement('div')
  probe.style.cssText = [
    'position:fixed',
    'top:0',
    'left:0',
    'width:0',
    'height:0',
    'visibility:hidden',
    'pointer-events:none',
    ...EDGES.map((e) => `padding-${e}:env(safe-area-inset-${e})`),
  ].join(';')
  document.body.appendChild(probe)
  const cs = getComputedStyle(probe)
  const out = {
    top: Math.round(Number.parseFloat(cs.paddingTop) || 0),
    right: Math.round(Number.parseFloat(cs.paddingRight) || 0),
    bottom: Math.round(Number.parseFloat(cs.paddingBottom) || 0),
    left: Math.round(Number.parseFloat(cs.paddingLeft) || 0),
  }
  probe.remove()
  return out
}

export type Insets = Record<(typeof EDGES)[number], number>

/**
 * The custom properties to write for a given inset reading, or `null` to write nothing.
 *
 * Split out from `apply()` so the rule below can be tested without a DOM — the reading itself
 * needs a browser, the decision does not.
 */
export function insetVars(inset: Insets, shortfall: number): Record<string, string> | null {
  // AN ALL-ZERO READ IS DISCARDED, and this is the whole reason the top bar once ended up
  // behind the status bar (task 145).
  //
  // The seeds in main.css are `env(safe-area-inset-*)`, which are **live**: the engine
  // re-substitutes them whenever the insets change. What we write here is a **static pixel
  // value** that shadows them. So writing a value we are not sure about is strictly worse
  // than writing nothing — it replaces a self-correcting declaration with a frozen wrong one.
  //
  // On iOS standalone the insets read as 0 until the first frame is painted, and this runs
  // before mount by design (see initSafeArea). The result was `--sat: 0px` written over a seed
  // that would have resolved to 59px moments later, permanently: no resize or orientation
  // change follows a normal cold launch, so nothing ever corrected it. Measured on the
  // maintainer's iPhone via the LayoutDebug overlay — raw `env()` 59/0/34/0 against our
  // effective values 0/0/0/0, with the shell's <main> starting at 69px, i.e. header content
  // with no inset at all.
  //
  // All four reading zero is exactly that state, and it is also the state of a device with
  // genuinely no insets (Android, desktop) — where leaving the seed alone is equally correct,
  // because the seed resolves to 0 too. So discarding an all-zero read costs nothing and is
  // the only case where the read carries no information the CSS does not already have.
  if (inset.top === 0 && inset.right === 0 && inset.bottom === 0 && inset.left === 0) {
    return null
  }

  return {
    '--sat': `${inset.top}px`,
    '--sar': `${inset.right}px`,
    // See the header note: reduced by the viewport's shortfall, which is geometry rather than
    // an inference about platform behaviour.
    '--sab': `${Math.max(0, inset.bottom - shortfall)}px`,
    '--sal': `${inset.left}px`,
  }
}

function apply() {
  // How far the reported viewport stops short of the screen's bottom edge. Clamped because
  // desktop reports a viewport taller than screen.height.
  const shortfall = Math.max(0, window.screen.height - window.innerHeight)

  const vars = insetVars(readInsets(), shortfall)
  if (vars === null) return

  const root = document.documentElement.style
  for (const [name, value] of Object.entries(vars)) {
    root.setProperty(name, value)
  }
}

let frame = 0
function schedule() {
  cancelAnimationFrame(frame)
  // Next frame: on a rotation the new geometry is not readable until it has been laid
  // out, and sampling synchronously yields the previous orientation's numbers.
  frame = requestAnimationFrame(apply)
}

export function initSafeArea() {
  // Synchronously, not via schedule(): initSafeArea runs before mount, and applying the
  // values in a rAF after first paint is what previously left the shell reflowing
  // underneath an already-measured scroll container. Insets are readable synchronously,
  // so there is no reason to defer the first pass.
  apply()

  // ...and once more after the first frame, because on iOS standalone the insets are not
  // readable before it (task 145). The synchronous pass above is kept: where the insets *are*
  // already available it applies the bottom correction before first paint, which is what
  // avoids the reflow described above. This second pass only changes anything when the first
  // one had nothing to work with, so it cannot reintroduce that reflow on a device that
  // answered immediately.
  schedule()
  window.addEventListener('resize', schedule)
  window.addEventListener('orientationchange', schedule)
  // Returning from a suspend can restore different chrome (iOS in particular), and a
  // standalone app is resumed far more often than it is loaded.
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) schedule()
  })
}
