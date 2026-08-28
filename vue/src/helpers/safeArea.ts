// Safe-area custom properties, resolved once and kept current.
//
// Components must use `var(--sat)` / `var(--sar)` / `var(--sab)` / `var(--sal)` rather
// than `env(safe-area-inset-*)` directly. main.css seeds them from `env()`, so the
// CSS-only behaviour is unchanged if this never runs; what this adds is one correction
// that cannot be expressed in CSS.
//
// THE PROBLEM
//
// `env(safe-area-inset-top)` answers "how much of the viewport is obscured by system
// chrome". It does NOT tell you whether the viewport even reaches that far. In an iOS
// standalone web app without a translucent status bar, the web view is laid out *below*
// the status bar — and iOS still reports a top inset of 59 anyway. Measured on an
// iPhone 16: screen 393x852, viewport 393x793, sa.top 59. Adding that inset there
// reserves 59px twice, which is exactly the blank band above the map, and the reason
// the map's controls sat ~59px lower than intended.
//
// The three states we have to be correct in:
//
//   iOS, translucent status bar   viewport covers screen   sa.top 59   inset needed
//   iOS, opaque status bar        viewport short by 59     sa.top 59   inset NOT needed
//   Android standalone            viewport short by ~24    sa.top 0    nothing to do
//
// THE RULE
//
// Only the top is special-cased, and only when the viewport's vertical shortfall is
// exactly the top inset — the signature of "iOS already moved us below the status bar".
// Deliberately not generalised to the other edges: the bottom inset stays necessary in
// that same state, because the view still extends under the home indicator, so zeroing
// `--sab` by the same reasoning would put the nav's last row under it. A single vertical
// shortfall cannot tell you which edge it came from, so it is only ever attributed to
// the edge whose inset it matches.
//
// Android needs no special case: it reports no top inset, so the correction is a no-op
// there and the same stylesheet is correct on both platforms.

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

/** True when the layout viewport actually spans the full screen height. */
export function viewportCoversScreen(): boolean {
  return window.screen.height - window.innerHeight <= 0
}

function apply() {
  const inset = readInsets()
  const shortfall = window.screen.height - window.innerHeight

  // The correction. `>0` guards the Android case, where shortfall and inset are both
  // small-or-zero and could coincide at 0 without meaning anything.
  const topAlreadyReserved = inset.top > 0 && shortfall === inset.top

  const root = document.documentElement.style
  root.setProperty('--sat', `${topAlreadyReserved ? 0 : inset.top}px`)
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
