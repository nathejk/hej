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
// THE BOTTOM INSET IS REDUCED BY THE VIEWPORT'S SHORTFALL, which is geometry rather than
// an inference about platform behaviour:
//
//   * `env(safe-area-inset-bottom)` exists to keep content clear of the home indicator at
//     the physical bottom of the screen.
//   * The viewport starts at screen y=0 (established above), so the whole of
//     `screen.height - innerHeight` is empty space at the BOTTOM.
//   * If the shell already ends 59px above the screen edge, it is already clear of a 34px
//     indicator. Padding the nav by another 34px just makes it look bloated — which is
//     what "the bottom nav has way too much padding" was.
//
// So the padding still needed is `max(0, inset.bottom - shortfall)`. This can only ever
// *reduce* padding that is provably unnecessary; when the viewport does fill the screen
// (shortfall 0) it is the full inset, unchanged. Android reports no bottom inset, so it is
// 0 either way.

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
  // Empty space below the shell, i.e. how far the viewport already stops short of the
  // screen's bottom edge. Clamped: a negative value (viewport taller than the screen,
  // which desktop reports) must not inflate the padding.
  const shortfall = Math.max(0, window.screen.height - window.innerHeight)

  const root = document.documentElement.style
  root.setProperty('--sat', `${inset.top}px`)
  root.setProperty('--sar', `${inset.right}px`)
  root.setProperty('--sab', `${Math.max(0, inset.bottom - shortfall)}px`)
  root.setProperty('--sal', `${inset.left}px`)
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
