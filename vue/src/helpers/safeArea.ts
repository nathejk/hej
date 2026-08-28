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
// THE BOTTOM INSET PASSES THROUGH UNCHANGED TOO. An earlier version reduced it by the
// viewport's shortfall, reasoning that a shell ending 59px above the screen edge was
// already clear of the 34px home indicator. True at the time, but it is the wrong fix for
// the wrong problem: the shell now extends to the physical bottom (see `.app-shell` in
// main.css), so the nav really does sit over the indicator and really does need the full
// 34px. Reducing it would put tap targets in the swipe-up area.
//
// THE ONE THING THIS COMPUTES is `--vh-extra`: the shortfall between the reported viewport
// height and the screen, which `.app-shell` adds to its own height so it reaches the
// bottom of the display.
//
// In an iOS standalone web app with a translucent status bar, `innerHeight` (and therefore
// `100dvh`) report `screen.height - topInset` while the web view is laid out from the very
// top of the screen. The shell consequently stops `topInset` short of the bottom, leaving a
// strip of blank background under the nav — the residue of task 036, which switching from
// `height: 100%` to `100dvh` reduced from ~100px to 59 without eliminating.
//
//   iOS, translucent status bar   short by sa.top   sa.top 59   extend the shell
//   Android standalone            short by ~24      sa.top 0    leave it alone
//   Browser tab / desktop         not short         sa.top 0    leave it alone
//
// The shortfall is only added back when it equals a non-zero top inset. That equality is
// what separates iOS from Android: Android is short too, by its status bar, but reports no
// top inset — and there the viewport genuinely does begin below the status bar, so
// extending the shell would push content underneath it. A bare "is the viewport short?"
// test would break Android.

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

function apply() {
  const inset = readInsets()
  // How far the reported viewport stops short of the screen's bottom edge. Clamped
  // because desktop reports a viewport taller than screen.height.
  const shortfall = Math.max(0, window.screen.height - window.innerHeight)
  const heightUnderReported = inset.top > 0 && shortfall === inset.top

  const root = document.documentElement.style
  root.setProperty('--sat', `${inset.top}px`)
  root.setProperty('--sar', `${inset.right}px`)
  root.setProperty('--sab', `${inset.bottom}px`)
  root.setProperty('--sal', `${inset.left}px`)
  root.setProperty('--vh-extra', `${heightUnderReported ? shortfall : 0}px`)
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
  window.addEventListener('resize', schedule)
  window.addEventListener('orientationchange', schedule)
  // Returning from a suspend can restore different chrome (iOS in particular), and a
  // standalone app is resumed far more often than it is loaded.
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) schedule()
  })
}
