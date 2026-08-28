// Safe-area custom properties, plus a viewport-height correction.
//
// Components must use `var(--sat)` / `var(--sar)` / `var(--sab)` / `var(--sal)` rather
// than `env(safe-area-inset-*)` directly, and the shell's height must include
// `var(--vh-extra)`. main.css seeds all of them, so the CSS-only behaviour is sane if
// this never runs.
//
// THE PROBLEM
//
// In an iOS standalone web app with a translucent status bar and `viewport-fit=cover`,
// the content genuinely extends to the top of the screen — but `window.innerHeight`
// (and therefore `100dvh`) reports the *safe-area* height, not the screen height.
// Measured on an iPhone 16: screen 393x852, innerHeight 793, sa.top 59, and
// `main.getBoundingClientRect().top === 0` with the map visibly drawing behind the
// status bar.
//
// So the viewport starts at the top of the screen and is 59px too short. A shell sized
// with `100dvh` ends 59px above the bottom of the display, which is the strip of blank
// background below the bottom nav (first seen in task 036, where switching from
// `height: 100%` to `100dvh` reduced it from ~100px to 59 without removing it).
//
// What this is NOT: a double-counted inset. `main.top` is 0, so nothing in our own
// layout reserves the status bar, and `--sat` is genuinely needed — it is what keeps the
// map controls clear of the clock. An earlier version of this file zeroed `--sat` on
// this evidence and was wrong; the insets are correct and the height is not.
//
// THE THREE STATES
//
//   iOS, translucent status bar   short by sa.top   sa.top 59   extend the shell
//   Android standalone            short by ~24      sa.top 0    leave everything alone
//   Browser tab / desktop         not short         sa.top 0    leave everything alone
//
// THE RULE
//
// Add the shortfall back to the shell height only when it is exactly the top inset and
// that inset is non-zero. That combination is the signature of "the view covers the
// screen but the reported height excludes the top inset".
//
// Android is short too — by its status bar — but reports no top inset, so the condition
// does not match and nothing is added. That matters: on Android the viewport really does
// begin below the status bar, so extending the shell there would push content underneath
// it. The equality test is what separates the two cases; a bare "is it short?" check
// would break Android.

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
  const shortfall = window.screen.height - window.innerHeight

  // See THE RULE above. The `> 0` term is what excludes Android, where shortfall and
  // inset would otherwise both be 0 and match meaninglessly.
  const heightUnderReported = inset.top > 0 && shortfall === inset.top

  const root = document.documentElement.style
  // Insets pass through untouched — they are correct as reported.
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
  schedule()
  window.addEventListener('resize', schedule)
  window.addEventListener('orientationchange', schedule)
  // Returning from a suspend can restore different chrome (iOS in particular), and a
  // standalone app is resumed far more often than it is loaded.
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) schedule()
  })
}
