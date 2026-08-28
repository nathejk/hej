// Safe-area custom properties.
//
// Components must use `var(--sat)` / `var(--sar)` / `var(--sab)` / `var(--sal)` rather
// than `env(safe-area-inset-*)` directly. main.css seeds them, so the CSS-only result is
// correct before this runs; the indirection exists so that a platform correction can be
// applied in one place instead of at a dozen call sites.
//
// The values are currently passed through UNCHANGED, and that is the conclusion of a
// wrong turn worth recording. Measured on an iPhone 16 standalone: screen 393x852,
// innerHeight 793, sa.top 59. That looks like a double-counted inset, and an earlier
// version zeroed `--sat` accordingly — but `main.getBoundingClientRect().top` was 0 with
// the map visibly drawing behind the status bar, which proves nothing in our layout
// reserves it. The insets are right; zeroing them would have pulled the map controls up
// under the clock.
//
// A second attempt then added the 59px shortfall to the shell's height to close the strip
// of background below the nav. That was also reverted: it gave the document scrollable
// overflow, so the whole shell could be dragged, and resizing the shell in a rAF after
// first paint left the scroll container in `main` mis-measured on a cold open. See the
// note in main.css.
//
// So this file deliberately does very little. It reads the insets and publishes them.
// Keep it that way unless a measurement — not an inference — says otherwise.

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
  const root = document.documentElement.style
  root.setProperty('--sat', `${inset.top}px`)
  root.setProperty('--sar', `${inset.right}px`)
  root.setProperty('--sab', `${inset.bottom}px`)
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
  schedule()
  window.addEventListener('resize', schedule)
  window.addEventListener('orientationchange', schedule)
  // Returning from a suspend can restore different chrome (iOS in particular), and a
  // standalone app is resumed far more often than it is loaded.
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) schedule()
  })
}
